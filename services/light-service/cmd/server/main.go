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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	grpcAdapter "github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/grpc"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/memory"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/mock"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/adapters/sqlite"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/domain"
	"github.com/quentinrf/plant-monitor/services/light-service/internal/ports"
	"github.com/quentinrf/plant-monitor/services/light-service/pkg/pb"
	"github.com/quentinrf/plant-monitor/services/light-service/pkg/tlsconfig"
)

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("starting light service")

	// Read configuration from environment
	config := loadConfig()

	// Initialize repository
	var repo domain.ReadingRepository
	switch config.RepoType {
	case "sqlite":
		r, err := sqlite.NewReadingRepository(config.DBPath)
		if err != nil {
			log.Fatal().Err(err).Str("db_path", config.DBPath).Msg("failed to open SQLite database")
		}
		defer r.Close()
		repo = r
		log.Info().Str("db_path", config.DBPath).Msg("initialized SQLite repository")
	default:
		repo = memory.NewReadingRepository()
		log.Info().Msg("initialized in-memory repository")
	}

	// Initialize sensor
	var sensor ports.LightSensor
	switch config.SensorType {
	case "gpio":
		log.Fatal().Msg("gpio sensor not yet implemented; set SENSOR_TYPE=mock")
	default:
		sensor = mock.NewFakeSensor(500.0, 100.0) // 500±100 lux (indoor lighting)
		log.Info().Msg("initialized mock sensor")
	}

	// Initialize gRPC handler
	handler := grpcAdapter.NewLightServiceHandler(repo, sensor)

	// Configure TLS if certificates are provided
	var serverOpts []grpc.ServerOption
	if config.TLSCert != "" {
		tlsCfg, err := tlsconfig.LoadServerTLS(config.TLSCert, config.TLSKey, config.TLSCA)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load TLS config")
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		log.Info().Msg("mTLS enabled")
	} else {
		log.Warn().Msg("TLS_CERT not set — starting without TLS (dev mode only)")
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(serverOpts...)
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
	RepoType       string // "memory" | "sqlite"
	DBPath         string // SQLite database file path (used when RepoType=sqlite)
	SensorType     string // "mock" | "gpio"
	TLSCert        string // path to this service's certificate
	TLSKey         string // path to this service's private key
	TLSCA          string // path to the CA certificate
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

	repoType := os.Getenv("REPO_TYPE")
	if repoType == "" {
		repoType = "memory"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./light.db"
	}

	sensorType := os.Getenv("SENSOR_TYPE")
	if sensorType == "" {
		sensorType = "mock"
	}

	return Config{
		Port:           port,
		RecordInterval: recordInterval,
		RepoType:       repoType,
		DBPath:         dbPath,
		SensorType:     sensorType,
		TLSCert:        os.Getenv("TLS_CERT"),
		TLSKey:         os.Getenv("TLS_KEY"),
		TLSCA:          os.Getenv("TLS_CA"),
	}
}
