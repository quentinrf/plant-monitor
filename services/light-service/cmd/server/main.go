package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcAdapter "github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/grpc"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/memory"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/mock"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/ports"
	"github.com/quentinrf/plant-monitor/services/light-service/pkg/pb"
)

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("starting light service")

	// Read configuration from environment
	config := loadConfig()

	// Initialize repository
	// For now, using in-memory - we'll swap to SQLite on Pi
	repo := memory.NewReadingRepository()
	log.Info().Msg("initialized in-memory repository")

	// Initialize sensor
	// For laptop: mock sensor
	// For Pi: GPIO sensor (we'll add this later)
	sensor := mock.NewFakeSensor(500.0, 100.0) // 500±100 lux (indoor lighting)
	log.Info().Msg("initialized mock sensor")

	// Initialize gRPC handler
	handler := grpcAdapter.NewLightServiceHandler(repo, sensor)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterLightServiceServer(grpcServer, handler)

	// Enable gRPC reflection for grpcurl testing
	reflection.Register(grpcServer)

	// Start gRPC server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", config.Port))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	log.Info().Str("port", config.Port).Msg("gRPC server listening")

	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal().Err(err).Msg("failed to serve")
		}
	}()

	// Start background recorder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recorder := ports.NewRecorder(sensor, repo, config.RecordInterval)
	go recorder.Start(ctx)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server...")

	// Graceful shutdown
	cancel() // Stop recorder
	grpcServer.GracefulStop()

	log.Info().Msg("server stopped")
}

// Config holds application configuration
type Config struct {
	Port           string
	RecordInterval time.Duration
}

// loadConfig reads configuration from environment variables
func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	recordInterval := 5 * time.Minute
	if intervalStr := os.Getenv("RECORD_INTERVAL"); intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			recordInterval = d
		}
	}

	return Config{
		Port:           port,
		RecordInterval: recordInterval,
	}
}
