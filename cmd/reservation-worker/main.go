package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/traffic-tacos/reservation-worker/internal/client"
	workerConfig "github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"github.com/traffic-tacos/reservation-worker/internal/server"
	"github.com/traffic-tacos/reservation-worker/internal/worker"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := workerConfig.Load()

	// Initialize logger
	logger, err := observability.NewLogger(cfg.LogLevel)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Initialize context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load secrets from AWS Secrets Manager if configured
	if err := cfg.MergeWithSecrets(ctx); err != nil {
		logger.Error("Failed to load secrets from AWS Secrets Manager", zap.Error(err))
		// Continue with default configuration
	}

	logger.Info("Starting reservation worker",
		zap.String("queue_url", cfg.SQSQueueURL),
		zap.Int("concurrency", cfg.WorkerConcurrency),
		zap.Int("max_retries", cfg.MaxRetries),
		zap.String("aws_profile", cfg.AWSProfile),
		zap.Bool("use_secret_manager", cfg.UseSecretManager),
	)

	// Initialize OpenTelemetry tracing (disabled for local development)
	// TODO: Fix schema conflict and re-enable
	/*
	tracingConfig := observability.TracingConfig{
		ServiceName:      "reservation-worker",
		ServiceVersion:   "1.0.0",
		Environment:      "production", // TODO: make configurable
		ExporterEndpoint: cfg.OTELExporterEndpoint,
	}

	tp, err := observability.InitTracing(ctx, tracingConfig)
	if err != nil {
		logger.Error("Failed to initialize tracing", zap.Error(err))
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown tracer provider", zap.Error(err))
		}
	}()
	*/

	// Initialize Prometheus metrics
	metrics := observability.NewMetrics()

	// Initialize AWS SDK
	awsOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.SQSRegion),
	}

	// Use AWS profile if specified
	if cfg.AWSProfile != "" {
		awsOpts = append(awsOpts, config.WithSharedConfigProfile(cfg.AWSProfile))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, awsOpts...)
	if err != nil {
		logger.Error("Failed to load AWS config", zap.Error(err))
		os.Exit(1)
	}

	sqsClient := sqs.NewFromConfig(awsCfg)

	// Initialize external service clients
	inventoryClient, err := client.NewInventoryClient(cfg.InventoryGRPCAddr)
	if err != nil {
		logger.Error("Failed to initialize inventory client", zap.Error(err))
		os.Exit(1)
	}
	defer inventoryClient.Close()

	reservationClient := client.NewReservationClient(cfg.ReservationAPIBase)

	// Initialize dispatcher with worker pool
	dispatcher := worker.NewDispatcher(
		cfg,
		inventoryClient,
		reservationClient,
		logger,
		metrics,
	)

	// Initialize SQS poller
	poller := worker.NewSQSPoller(
		sqsClient,
		cfg,
		logger,
		metrics,
		dispatcher.GetEventsChan(),
	)

	// Start HTTP server for health checks and metrics
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startHTTPServer(cfg.ServerPort, logger)
	}()

	// Start gRPC server for debugging (grpcui support)
	grpcPort, err := strconv.Atoi(cfg.GRPCDebugPort)
	if err != nil {
		logger.Error("Invalid gRPC debug port", zap.Error(err))
		os.Exit(1)
	}

	grpcServer := server.NewGRPCServer(grpcPort, logger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcServer.Start(ctx); err != nil {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// Start dispatcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := dispatcher.Start(ctx); err != nil {
			logger.Error("Dispatcher failed", zap.Error(err))
		}
	}()

	// Start SQS poller
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := poller.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("SQS poller failed", zap.Error(err))
		}
	}()

	logger.Info("Reservation worker started successfully")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Received shutdown signal, shutting down gracefully...")

	// Cancel context to signal shutdown
	cancel()

	// Stop components
	poller.Stop()
	dispatcher.Stop()
	grpcServer.Stop()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Graceful shutdown completed")
	case <-time.After(30 * time.Second):
		logger.Warn("Shutdown timeout exceeded, forcing exit")
	}
}

// startHTTPServer starts HTTP server for health checks and metrics
func startHTTPServer(port string, logger *observability.Logger) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness check endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	})

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	logger.Info("Starting HTTP server", zap.String("port", port))

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server failed", zap.Error(err))
	}
}