package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the reservation worker
type Config struct {
	// SQS Configuration
	SQSQueueURL string `env:"SQS_QUEUE_URL,required"`
	SQSWaitTime int    `env:"SQS_WAIT_TIME" envDefault:"20"`

	// Worker Configuration
	WorkerConcurrency int `env:"WORKER_CONCURRENCY" envDefault:"20"`

	// Retry Configuration
	MaxRetries    int `env:"MAX_RETRIES" envDefault:"5"`
	BackoffBaseMs int `env:"BACKOFF_BASE_MS" envDefault:"1000"`

	// Service Addresses
	InventoryGRPCAddr  string `env:"INVENTORY_GRPC_ADDR,required"`
	ReservationAPIBase string `env:"RESERVATION_API_BASE,required"`

	// Observability
	OtelExporterOTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"http://otel-collector:4317"`
	LogLevel                 string `env:"LOG_LEVEL" envDefault:"info"`

	// Derived fields
	BackoffBaseDuration time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Convert backoff base ms to duration
	cfg.BackoffBaseDuration = time.Duration(cfg.BackoffBaseMs) * time.Millisecond

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.SQSQueueURL == "" {
		return fmt.Errorf("SQS_QUEUE_URL is required")
	}
	if c.SQSWaitTime <= 0 || c.SQSWaitTime > 20 {
		return fmt.Errorf("SQS_WAIT_TIME must be between 1 and 20 seconds")
	}
	if c.WorkerConcurrency <= 0 || c.WorkerConcurrency > 1000 {
		return fmt.Errorf("WORKER_CONCURRENCY must be between 1 and 1000")
	}
	if c.MaxRetries < 0 || c.MaxRetries > 10 {
		return fmt.Errorf("MAX_RETRIES must be between 0 and 10")
	}
	if c.BackoffBaseMs < 100 || c.BackoffBaseMs > 10000 {
		return fmt.Errorf("BACKOFF_BASE_MS must be between 100 and 10000 ms")
	}
	if c.InventoryGRPCAddr == "" {
		return fmt.Errorf("INVENTORY_GRPC_ADDR is required")
	}
	if c.ReservationAPIBase == "" {
		return fmt.Errorf("RESERVATION_API_BASE is required")
	}
	if c.LogLevel != "debug" && c.LogLevel != "info" && c.LogLevel != "warn" && c.LogLevel != "error" {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error")
	}

	return nil
}

// String returns a string representation of the config (without sensitive data)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{SQSQueueURL: %s, SQSWaitTime: %d, WorkerConcurrency: %d, MaxRetries: %d, BackoffBaseMs: %d, InventoryGRPCAddr: %s, ReservationAPIBase: %s, OtelExporterOTLPEndpoint: %s, LogLevel: %s}",
		c.SQSQueueURL,
		c.SQSWaitTime,
		c.WorkerConcurrency,
		c.MaxRetries,
		c.BackoffBaseMs,
		c.InventoryGRPCAddr,
		c.ReservationAPIBase,
		c.OtelExporterOTLPEndpoint,
		c.LogLevel,
	)
}

// GetEnvMap returns a map of all environment variables that should be set
func GetEnvMap() map[string]string {
	return map[string]string{
		"SQS_QUEUE_URL":               "https://sqs.ap-northeast-2.amazonaws.com/123/queue",
		"SQS_WAIT_TIME":               "20",
		"WORKER_CONCURRENCY":          "20",
		"MAX_RETRIES":                 "5",
		"BACKOFF_BASE_MS":             "1000",
		"INVENTORY_GRPC_ADDR":         "inventory-svc:8080",
		"RESERVATION_API_BASE":        "http://reservation-api:8080",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel-collector:4317",
		"LOG_LEVEL":                   "info",
	}
}

// GetBackoffDuration calculates the backoff duration for a given attempt
func (c *Config) GetBackoffDuration(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	return c.BackoffBaseDuration * time.Duration(1<<attempt)
}

// IsMaxRetriesReached checks if the maximum number of retries has been reached
func (c *Config) IsMaxRetriesReached(attempt int) bool {
	return attempt >= c.MaxRetries
}
