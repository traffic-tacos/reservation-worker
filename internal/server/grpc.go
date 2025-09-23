package server

import (
	"context"
	"fmt"
	"net"

	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// GRPCServer wraps the gRPC server for debugging and health checks
type GRPCServer struct {
	server *grpc.Server
	logger *observability.Logger
	port   int
}

// NewGRPCServer creates a new gRPC server with health check and reflection
func NewGRPCServer(port int, logger *observability.Logger) *GRPCServer {
	// Create gRPC server with OpenTelemetry instrumentation
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("reservation-worker", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service for grpcui debugging (only in development)
	reflection.Register(server)

	return &GRPCServer{
		server: server,
		logger: logger,
		port:   port,
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	s.logger.Info("Starting gRPC server for debugging", zap.Int("port", s.port))

	// Start server in a goroutine
	go func() {
		if err := s.server.Serve(lis); err != nil {
			s.logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	s.logger.Info("Shutting down gRPC server")
	s.server.GracefulStop()

	return nil
}

// Stop stops the gRPC server
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}