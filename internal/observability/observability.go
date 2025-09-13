package observability

import (
	"context"

	"github.com/traffic-tacos/reservation-worker/internal/config"
)

// Observability holds all observability components
type Observability struct {
	Logger  *Logger
	Metrics *Metrics
	Config  *config.Config
}

// New creates a new observability instance
func New(cfg *config.Config) (*Observability, error) {
	logger, err := NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	metrics := NewMetrics()

	return &Observability{
		Logger:  logger,
		Metrics: metrics,
		Config:  cfg,
	}, nil
}

// InitTracing initializes OpenTelemetry tracing and metrics
func (o *Observability) InitTracing(ctx context.Context) error {
	// Setup tracing
	traceProvider, err := SetupTracing(ctx, o.Config.OtelExporterOTLPEndpoint)
	if err != nil {
		o.Logger.ErrorLog(ctx, err, map[string]interface{}{
			"component": "tracing",
			"endpoint":  o.Config.OtelExporterOTLPEndpoint,
		}).Error("Failed to setup tracing")
		return err
	}

	// Setup metrics
	_, err = SetupMetrics(ctx, o.Config.OtelExporterOTLPEndpoint)
	if err != nil {
		o.Logger.ErrorLog(ctx, err, map[string]interface{}{
			"component": "metrics",
			"endpoint":  o.Config.OtelExporterOTLPEndpoint,
		}).Error("Failed to setup metrics")
		return err
	}

	// Setup runtime metrics
	if err := SetupRuntimeMetrics(ctx); err != nil {
		o.Logger.ErrorLog(ctx, err, map[string]interface{}{
			"component": "runtime_metrics",
		}).Warn("Failed to setup runtime metrics, continuing...")
		// Don't return error for runtime metrics failure
	}

	o.Logger.Info("Observability initialized successfully")

	// Store providers for cleanup (if needed)
	_ = traceProvider

	return nil
}

// Shutdown gracefully shuts down observability components
func (o *Observability) Shutdown(ctx context.Context) error {
	o.Logger.Info("Shutting down observability components")

	// Note: In a real implementation, you might want to properly shutdown
	// trace and meter providers here

	return nil
}
