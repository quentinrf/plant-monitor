package grpc

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quentinrf/plant-monitor/services/plant-service/internal/domain"
	"github.com/quentinrf/plant-monitor/services/plant-service/internal/ports"
	"github.com/quentinrf/plant-monitor/services/plant-service/pkg/pb"
)

// PlantServiceHandler implements the gRPC PlantService server.
type PlantServiceHandler struct {
	pb.UnimplementedPlantServiceServer
	lightClient ports.LightClient
}

// NewPlantServiceHandler creates the handler wired to the given LightClient.
func NewPlantServiceHandler(lightClient ports.LightClient) *PlantServiceHandler {
	return &PlantServiceHandler{lightClient: lightClient}
}

// GetPlantStatus fetches the current lux reading and the last hour of history,
// runs domain analysis, and returns a PlantStatus.
func (h *PlantServiceHandler) GetPlantStatus(ctx context.Context, _ *pb.GetPlantStatusRequest) (*pb.GetPlantStatusResponse, error) {
	log.Info().Msg("GetPlantStatus called")

	current, err := h.lightClient.GetCurrentLux(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to get current lux")
		return nil, status.Errorf(codes.Unavailable, "light-service unavailable: %v", err)
	}

	now := time.Now()
	history, err := h.lightClient.GetHistory(ctx, now.Add(-time.Hour), now)
	if err != nil {
		log.Warn().Err(err).Msg("could not fetch history for status; using current reading only")
		history = nil
	}

	analysis := domain.Analyze(current.Lux, luxSlice(history))

	return &pb.GetPlantStatusResponse{
		Status: &pb.PlantStatus{
			Recommendation: analysis.Recommendation,
			LightCategory:  analysis.Category,
			CurrentLux:     analysis.CurrentLux,
			Trend:          analysis.Trend,
			Timestamp:      current.Timestamp.Unix(),
		},
	}, nil
}

// GetHistory fetches readings for the requested time range, maps them to
// HistoryPoints, and computes the overall trend.
func (h *PlantServiceHandler) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	log.Info().Int64("start", req.StartTime).Int64("end", req.EndTime).Msg("GetHistory called")

	start := time.Unix(req.StartTime, 0)
	end := time.Unix(req.EndTime, 0)

	readings, err := h.lightClient.GetHistory(ctx, start, end)
	if err != nil {
		log.Error().Err(err).Msg("failed to get history from light-service")
		return nil, status.Errorf(codes.Unavailable, "light-service unavailable: %v", err)
	}

	points := make([]*pb.HistoryPoint, len(readings))
	for i, r := range readings {
		points[i] = &pb.HistoryPoint{
			Timestamp: r.Timestamp.Unix(),
			Lux:       r.Lux,
			Category:  r.Category,
		}
	}

	analysis := domain.Analyze(0, luxSlice(readings))

	return &pb.GetHistoryResponse{
		Points: points,
		Trend:  analysis.Trend,
	}, nil
}

// luxSlice extracts the lux values from a slice of LightReadings.
func luxSlice(readings []ports.LightReading) []float64 {
	lux := make([]float64, len(readings))
	for i, r := range readings {
		lux[i] = r.Lux
	}
	return lux
}
