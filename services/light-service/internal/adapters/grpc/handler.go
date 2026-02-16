package grpc

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/ports"
	"github.com/quentinrf/plant-monitor/services/light-service/pkg/pb"
)

// LightServiceHandler implements the gRPC LightService
type LightServiceHandler struct {
	pb.UnimplementedLightServiceServer
	repo   domain.ReadingRepository
	sensor ports.LightSensor
}

// NewLightServiceHandler creates a new gRPC handler
func NewLightServiceHandler(repo domain.ReadingRepository, sensor ports.LightSensor) *LightServiceHandler {
	return &LightServiceHandler{
		repo:   repo,
		sensor: sensor,
	}
}

// GetCurrentLight returns the most recent reading
func (h *LightServiceHandler) GetCurrentLight(ctx context.Context, req *pb.GetCurrentLightRequest) (*pb.GetCurrentLightResponse, error) {
	log.Info().Msg("GetCurrentLight called")

	reading, err := h.repo.GetLatestReading(ctx)
	if err == domain.ErrReadingNotFound {
		// No readings yet - read sensor now
		log.Info().Msg("no readings in database, reading sensor")

		lux, err := h.sensor.ReadLux(ctx)
		if err != nil {
			log.Error().Err(err).Msg("failed to read sensor")
			return nil, status.Error(codes.Internal, "failed to read sensor")
		}

		reading, err = domain.NewLightReading(lux)
		if err != nil {
			log.Error().Err(err).Msg("failed to create reading")
			return nil, status.Error(codes.Internal, "failed to create reading")
		}

		// Save for next time
		if err := h.repo.SaveReading(ctx, reading); err != nil {
			log.Error().Err(err).Msg("failed to save reading")
			// Don't fail - we still have the reading
		}
	} else if err != nil {
		log.Error().Err(err).Msg("failed to get latest reading")
		return nil, status.Error(codes.Internal, "failed to get reading")
	}

	return &pb.GetCurrentLightResponse{
		Reading: convertReadingToProto(reading),
	}, nil
}

// GetHistory returns readings within time range with statistics
func (h *LightServiceHandler) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error) {
	log.Info().
		Int64("start", req.StartTime).
		Int64("end", req.EndTime).
		Msg("GetHistory called")

	start := time.Unix(req.StartTime, 0)
	end := time.Unix(req.EndTime, 0)

	readings, err := h.repo.GetReadingsInRange(ctx, start, end)
	if err != nil {
		log.Error().Err(err).Msg("failed to get readings")
		return nil, status.Error(codes.Internal, "failed to get readings")
	}

	// Convert to protobuf
	pbReadings := make([]*pb.LightReading, len(readings))
	for i, r := range readings {
		pbReadings[i] = convertReadingToProto(r)
	}

	// Calculate statistics
	stats := calculateStatistics(readings)

	return &pb.GetHistoryResponse{
		Readings:   pbReadings,
		AverageLux: stats.average,
		MinLux:     stats.min,
		MaxLux:     stats.max,
	}, nil
}

// RecordReading manually records a reading (useful for testing)
func (h *LightServiceHandler) RecordReading(ctx context.Context, req *pb.RecordReadingRequest) (*pb.RecordReadingResponse, error) {
	log.Info().Float64("lux", req.Lux).Msg("RecordReading called")

	reading, err := domain.NewLightReading(req.Lux)
	if err != nil {
		log.Error().Err(err).Msg("invalid lux value")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := h.repo.SaveReading(ctx, reading); err != nil {
		log.Error().Err(err).Msg("failed to save reading")
		return nil, status.Error(codes.Internal, "failed to save reading")
	}

	return &pb.RecordReadingResponse{
		Reading: convertReadingToProto(reading),
	}, nil
}

// convertReadingToProto converts domain model to protobuf
func convertReadingToProto(r *domain.LightReading) *pb.LightReading {
	return &pb.LightReading{
		Id:        r.ID,
		Lux:       r.Lux,
		Timestamp: r.Timestamp.Unix(),
		Category:  r.LightCategory(),
	}
}

// statistics holds calculated statistics
type statistics struct {
	average float64
	min     float64
	max     float64
}

// calculateStatistics computes stats for a set of readings
func calculateStatistics(readings []*domain.LightReading) statistics {
	if len(readings) == 0 {
		return statistics{}
	}

	var sum float64
	min := readings[0].Lux
	max := readings[0].Lux

	for _, r := range readings {
		sum += r.Lux
		if r.Lux < min {
			min = r.Lux
		}
		if r.Lux > max {
			max = r.Lux
		}
	}

	return statistics{
		average: sum / float64(len(readings)),
		min:     min,
		max:     max,
	}
}
