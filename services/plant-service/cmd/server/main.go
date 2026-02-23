package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	grpcAdapter "github.com/quentinrf/plant-monitor/services/plant-service/internal/adapters/grpc"
	"github.com/quentinrf/plant-monitor/services/plant-service/pkg/pb"
	"github.com/quentinrf/plant-monitor/services/plant-service/pkg/tlsconfig"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("starting plant-service")

	config := loadConfig()

	// Build the TLS config for the outbound call to light-service (client role).
	var lightTLSCfg *tls.Config
	if config.TLSCert != "" {
		cfg, err := tlsconfig.LoadClientTLS(config.TLSCert, config.TLSKey, config.TLSCA)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load client TLS config")
		}
		lightTLSCfg = cfg
		log.Info().Msg("mTLS enabled for light-service connection")
	} else {
		log.Warn().Msg("TLS_CERT not set — connecting to light-service without TLS (dev mode only)")
	}

	// Connect to light-service.
	lightClient, err := grpcAdapter.NewLightClientAdapter(config.LightServiceAddr, lightTLSCfg)
	if err != nil {
		log.Fatal().Err(err).Str("addr", config.LightServiceAddr).Msg("failed to connect to light-service")
	}
	defer lightClient.Close()

	log.Info().Str("addr", config.LightServiceAddr).Msg("connected to light-service")

	// Build gRPC server — mTLS if certs provided, insecure otherwise.
	handler := grpcAdapter.NewPlantServiceHandler(lightClient)

	var serverOpts []grpc.ServerOption
	if config.TLSCert != "" {
		tlsCfg, err := tlsconfig.LoadServerTLS(config.TLSCert, config.TLSKey, config.TLSCA)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load server TLS config")
		}
		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
		log.Info().Msg("mTLS enabled for incoming connections")
	} else {
		log.Warn().Msg("starting gRPC server without TLS (dev mode only)")
	}

	grpcServer := grpc.NewServer(serverOpts...)
	pb.RegisterPlantServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", config.Port))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to listen")
	}

	log.Info().Str("port", config.Port).Msg("gRPC server listening")

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatal().Err(err).Msg("failed to serve")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down plant-service...")

	_, cancel := context.WithCancel(context.Background())
	cancel()
	grpcServer.GracefulStop()

	log.Info().Msg("plant-service stopped")
}

// Config holds application configuration read from environment variables.
type Config struct {
	Port             string
	LightServiceAddr string
	TLSCert          string
	TLSKey           string
	TLSCA            string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50052"
	}

	lightAddr := os.Getenv("LIGHT_SERVICE_ADDR")
	if lightAddr == "" {
		lightAddr = "localhost:50051"
	}

	return Config{
		Port:             port,
		LightServiceAddr: lightAddr,
		TLSCert:          os.Getenv("TLS_CERT"),
		TLSKey:           os.Getenv("TLS_KEY"),
		TLSCA:            os.Getenv("TLS_CA"),
	}
}
