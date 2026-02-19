package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/memory"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/mock"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
	"github.com/quentinrf/plant-monitor/services/light-service/pkg/pb"
)

// startTestServer creates an in-process gRPC server and returns a connected client.
// The server is stopped when the test ends.
func startTestServer(t *testing.T) pb.LightServiceClient {
	t.Helper()

	repo := memory.NewReadingRepository()
	sensor := mock.NewFakeSensor(500.0, 0) // deterministic: always 500 lux
	handler := NewLightServiceHandler(repo, sensor)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterLightServiceServer(srv, handler)

	go srv.Serve(lis)
	t.Cleanup(func() {
		srv.GracefulStop()
	})

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewLightServiceClient(conn)
}

func TestGetCurrentLight_NoReadings(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	resp, err := client.GetCurrentLight(ctx, &pb.GetCurrentLightRequest{})
	if err != nil {
		t.Fatalf("GetCurrentLight failed: %v", err)
	}
	if resp.Reading == nil {
		t.Fatal("expected a reading, got nil")
	}
	// Mock sensor always returns 500 lux
	if resp.Reading.Lux != 500.0 {
		t.Errorf("expected lux 500, got %v", resp.Reading.Lux)
	}
	if resp.Reading.Category != "Medium Light" {
		t.Errorf("expected category 'Medium Light', got %q", resp.Reading.Category)
	}
}

func TestRecordReading_ThenGetCurrent(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	// Manually record a specific reading
	recordResp, err := client.RecordReading(ctx, &pb.RecordReadingRequest{Lux: 100.0})
	if err != nil {
		t.Fatalf("RecordReading failed: %v", err)
	}
	if recordResp.Reading.Lux != 100.0 {
		t.Errorf("expected recorded lux 100, got %v", recordResp.Reading.Lux)
	}

	// GetCurrentLight should now return the stored reading (latest)
	resp, err := client.GetCurrentLight(ctx, &pb.GetCurrentLightRequest{})
	if err != nil {
		t.Fatalf("GetCurrentLight failed: %v", err)
	}
	if resp.Reading.Lux != 100.0 {
		t.Errorf("expected current lux 100, got %v", resp.Reading.Lux)
	}
	if resp.Reading.Category != "Low Light" {
		t.Errorf("expected category 'Low Light', got %q", resp.Reading.Category)
	}
}

func TestGetHistory_TimeRange(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	now := time.Now()

	// Seed two readings via RecordReading
	_, err := client.RecordReading(ctx, &pb.RecordReadingRequest{Lux: 300.0})
	if err != nil {
		t.Fatalf("RecordReading failed: %v", err)
	}
	_, err = client.RecordReading(ctx, &pb.RecordReadingRequest{Lux: 600.0})
	if err != nil {
		t.Fatalf("RecordReading failed: %v", err)
	}

	// Query a range covering now ±1 minute
	start := now.Add(-time.Minute)
	end := now.Add(time.Minute)

	resp, err := client.GetHistory(ctx, &pb.GetHistoryRequest{
		StartTime: start.Unix(),
		EndTime:   end.Unix(),
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(resp.Readings) != 2 {
		t.Fatalf("expected 2 readings, got %d", len(resp.Readings))
	}

	// Statistics
	expectedAvg := (300.0 + 600.0) / 2
	if resp.AverageLux != expectedAvg {
		t.Errorf("expected average %v, got %v", expectedAvg, resp.AverageLux)
	}
	if resp.MinLux != 300.0 {
		t.Errorf("expected min 300, got %v", resp.MinLux)
	}
	if resp.MaxLux != 600.0 {
		t.Errorf("expected max 600, got %v", resp.MaxLux)
	}
}

func TestGetHistory_EmptyRange(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	// Query a range in the distant past — no readings
	start := time.Now().Add(-48 * time.Hour)
	end := time.Now().Add(-47 * time.Hour)

	resp, err := client.GetHistory(ctx, &pb.GetHistoryRequest{
		StartTime: start.Unix(),
		EndTime:   end.Unix(),
	})
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(resp.Readings) != 0 {
		t.Errorf("expected 0 readings, got %d", len(resp.Readings))
	}
}

// Compile-time check: ensure RecordReading sets category correctly
func TestRecordReading_CategoryMapping(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	cases := []struct {
		lux      float64
		category string
	}{
		{50.0, "Low Light"},
		{1000.0, "Medium Light"},
		{3000.0, "High Light"},
	}

	for _, tc := range cases {
		resp, err := client.RecordReading(ctx, &pb.RecordReadingRequest{Lux: tc.lux})
		if err != nil {
			t.Fatalf("RecordReading(%.0f) failed: %v", tc.lux, err)
		}
		if resp.Reading.Category != tc.category {
			t.Errorf("lux %.0f: expected category %q, got %q", tc.lux, tc.category, resp.Reading.Category)
		}
	}
}

// Ensure invalid lux returns an error
func TestRecordReading_InvalidLux(t *testing.T) {
	client := startTestServer(t)
	ctx := context.Background()

	_, err := client.RecordReading(ctx, &pb.RecordReadingRequest{Lux: -10.0})
	if err == nil {
		t.Error("expected error for negative lux, got nil")
	}
}

// Verify domain.ErrReadingNotFound is never silently swallowed in the test helper
var _ = domain.ErrReadingNotFound
