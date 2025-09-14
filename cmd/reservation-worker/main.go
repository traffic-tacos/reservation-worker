// Package main implements the Reservation Worker service
//
// Reservation Worker is a high-performance event processing service for Traffic Tacos
// that handles reservation lifecycle events from SQS, ensuring reliable inventory
// management and order processing.
//
// Terms Of Service: http://swagger.io/terms/
//
// Schemes: http, https
// Host: localhost:8080
// BasePath: /
// Version: 1.0.0
//
// Consumes:
// - application/json
//
// Produces:
// - application/json
//
// swagger:meta
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	_ "github.com/traffic-tacos/reservation-worker/docs" // This is required for Swagger

	"github.com/traffic-tacos/reservation-worker/internal/client"
	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/handler"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"github.com/traffic-tacos/reservation-worker/internal/server"
	"github.com/traffic-tacos/reservation-worker/internal/worker"
)

func main() {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("Starting reservation worker with config: %s\n", cfg.String())

	// Initialize observability
	obs, err := observability.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}

	// Initialize OpenTelemetry
	if err := obs.InitTracing(ctx); err != nil {
		obs.Logger.ErrorLog(ctx, err, nil).Fatal("Failed to initialize tracing")
	}

	obs.Logger.Info("Observability initialized")

	// Initialize clients
	inventoryClient, err := client.NewInventoryServiceClient(cfg.InventoryGRPCAddr, obs.Logger)
	if err != nil {
		obs.Logger.ErrorLog(ctx, err, nil).Fatal("Failed to create inventory client")
	}
	defer inventoryClient.Close()

	reservationClient := client.NewReservationServiceClient(cfg.ReservationAPIBase, obs.Logger)

	// Initialize event handler
	eventHandler := handler.NewEventHandler(
		inventoryClient,
		reservationClient,
		obs.Logger,
		obs.Metrics,
	)

	// Initialize worker
	w, err := worker.NewWorker(cfg, eventHandler, obs.Logger, obs.Metrics)
	if err != nil {
		obs.Logger.ErrorLog(ctx, err, nil).Fatal("Failed to create worker")
	}

	// Start worker
	if err := w.Start(ctx); err != nil {
		obs.Logger.ErrorLog(ctx, err, nil).Fatal("Failed to start worker")
	}

	// Initialize HTTP server
	httpServer := server.NewServer(cfg, obs.Logger)

	// Start HTTP server
	if err := httpServer.Start(ctx); err != nil {
		obs.Logger.ErrorLog(ctx, err, nil).Fatal("Failed to start HTTP server")
	}

	obs.Logger.Info("Reservation worker started successfully")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		obs.Logger.Logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		obs.Logger.Info("Context cancelled")
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	obs.Logger.Info("Initiating graceful shutdown")

	// Stop worker
	if err := w.Stop(shutdownCtx); err != nil {
		obs.Logger.ErrorLog(shutdownCtx, err, nil).Error("Error stopping worker")
	}

	// Stop HTTP server
	if err := httpServer.Stop(shutdownCtx); err != nil {
		obs.Logger.ErrorLog(shutdownCtx, err, nil).Error("Error stopping HTTP server")
	}

	// Shutdown observability
	if err := obs.Shutdown(shutdownCtx); err != nil {
		obs.Logger.ErrorLog(shutdownCtx, err, nil).Error("Error shutting down observability")
	}

	obs.Logger.Info("Reservation worker shutdown complete")
}
