package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quentinrf/plant-monitor/services/plant-service/internal/ports"

	lightpb "github.com/quentinrf/plant-monitor/services/light-service/pkg/pb"
)

// LightClientAdapter implements ports.LightClient by calling light-service over gRPC.
type LightClientAdapter struct {
	conn   *grpc.ClientConn
	client lightpb.LightServiceClient
}

// NewLightClientAdapter dials light-service. Pass nil tlsConfig for insecure (dev) mode.
func NewLightClientAdapter(addr string, tlsConfig *tls.Config) (*LightClientAdapter, error) {
	var dialOpt grpc.DialOption
	if tlsConfig != nil {
		dialOpt = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	} else {
		dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(addr, dialOpt)
	if err != nil {
		return nil, fmt.Errorf("dial light-service at %s: %w", addr, err)
	}

	return &LightClientAdapter{
		conn:   conn,
		client: lightpb.NewLightServiceClient(conn),
	}, nil
}

// GetCurrentLux fetches the most recent lux reading from light-service.
func (a *LightClientAdapter) GetCurrentLux(ctx context.Context) (*ports.LightReading, error) {
	resp, err := a.client.GetCurrentLight(ctx, &lightpb.GetCurrentLightRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetCurrentLight: %w", err)
	}

	r := resp.GetReading()
	return &ports.LightReading{
		Lux:       r.GetLux(),
		Timestamp: time.Unix(r.GetTimestamp(), 0),
		Category:  r.GetCategory(),
	}, nil
}

// GetHistory fetches readings from light-service in the half-open interval [start, end).
func (a *LightClientAdapter) GetHistory(ctx context.Context, start, end time.Time) ([]ports.LightReading, error) {
	resp, err := a.client.GetHistory(ctx, &lightpb.GetHistoryRequest{
		StartTime: start.Unix(),
		EndTime:   end.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("GetHistory: %w", err)
	}

	readings := make([]ports.LightReading, len(resp.GetReadings()))
	for i, r := range resp.GetReadings() {
		readings[i] = ports.LightReading{
			Lux:       r.GetLux(),
			Timestamp: time.Unix(r.GetTimestamp(), 0),
			Category:  r.GetCategory(),
		}
	}
	return readings, nil
}

// Close releases the underlying gRPC connection.
func (a *LightClientAdapter) Close() error {
	return a.conn.Close()
}
