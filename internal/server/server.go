package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.uber.org/zap"

	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
)

// Server represents the HTTP server
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *observability.Logger
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, logger *observability.Logger) *Server {
	e := echo.New()

	// Hide Echo banner
	e.HideBanner = true
	e.HidePort = true

	// Add middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	server := &Server{
		echo:   e,
		config: cfg,
		logger: logger,
	}

	// Register routes
	server.registerRoutes()

	return server
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Health check endpoint
	s.echo.GET("/health", s.healthCheck)

	// Readiness check endpoint
	s.echo.GET("/ready", s.readinessCheck)

	// Metrics endpoint for Prometheus
	s.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// Swagger documentation
	s.echo.GET("/swagger/*", echoSwagger.WrapHandler)

	// API v1 routes
	v1 := s.echo.Group("/api/v1")
	{
		v1.GET("/status", s.getStatus)
		v1.GET("/config", s.getConfig)
	}
}

// healthCheck handles health check requests
// @Summary Health check endpoint
// @Description Check if the service is healthy
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (s *Server) healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "reservation-worker",
	})
}

// readinessCheck handles readiness check requests
// @Summary Readiness check endpoint
// @Description Check if the service is ready to handle requests
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ready [get]
func (s *Server) readinessCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "reservation-worker",
	})
}

// getStatus returns service status information
// @Summary Get service status
// @Description Get detailed service status and metrics
// @Tags status
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/status [get]
func (s *Server) getStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"service":            "reservation-worker",
		"version":            "1.0.0",
		"status":             "running",
		"timestamp":          time.Now().UTC().Format(time.RFC3339),
		"worker_concurrency": s.config.WorkerConcurrency,
		"max_retries":        s.config.MaxRetries,
		"sqs_wait_time":      s.config.SQSWaitTime,
		"log_level":          s.config.LogLevel,
	})
}

// getConfig returns service configuration (without sensitive data)
// @Summary Get service configuration
// @Description Get service configuration information (safe for exposure)
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/config [get]
func (s *Server) getConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"service":            "reservation-worker",
		"worker_concurrency": s.config.WorkerConcurrency,
		"max_retries":        s.config.MaxRetries,
		"sqs_wait_time":      s.config.SQSWaitTime,
		"http_port":          s.config.HTTPPort,
		"log_level":          s.config.LogLevel,
		"backoff_base_ms":    s.config.BackoffBaseMs,
	})
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Logger.Info("Starting HTTP server", zap.String("port", s.config.HTTPPort))

	// Start server in a goroutine
	go func() {
		addr := fmt.Sprintf(":%s", s.config.HTTPPort)
		if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
			s.logger.Logger.Error("HTTP server failed to start", zap.Error(err))
		}
	}()

	s.logger.Logger.Info("HTTP server started successfully",
		zap.String("port", s.config.HTTPPort),
		zap.String("swagger_url", fmt.Sprintf("http://localhost:%s/swagger/index.html", s.config.HTTPPort)))

	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Logger.Info("Stopping HTTP server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.echo.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	s.logger.Logger.Info("HTTP server stopped gracefully")
	return nil
}
