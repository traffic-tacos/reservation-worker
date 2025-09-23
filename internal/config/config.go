package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the reservation worker
type Config struct {
	// AWS Configuration
	AWSProfile      string
	AWSRegion       string
	UseSecretManager bool
	SecretName      string

	// SQS Configuration
	SQSQueueURL  string
	SQSWaitTime  int
	SQSRegion    string

	// Worker Configuration
	WorkerConcurrency int
	MaxRetries        int
	BackoffBaseMS     int

	// External Services
	InventoryGRPCAddr    string
	ReservationAPIBase   string

	// Observability
	OTELExporterEndpoint string
	LogLevel             string

	// Server Configuration
	ServerPort     string // HTTP server for health/metrics
	GRPCDebugPort  string // gRPC server for debugging
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		// AWS Configuration
		AWSProfile:       getEnv("AWS_PROFILE", ""),
		AWSRegion:        getEnv("AWS_REGION", "ap-northeast-2"),
		UseSecretManager: getEnvBool("USE_SECRET_MANAGER", false),
		SecretName:       getEnv("SECRET_NAME", "traffictacos/reservation-worker"),

		// SQS Configuration
		SQSQueueURL:  getEnv("SQS_QUEUE_URL", "https://sqs.ap-northeast-2.amazonaws.com/123/reservation-events"),
		SQSWaitTime:  getEnvInt("SQS_WAIT_TIME", 20),
		SQSRegion:    getEnv("AWS_REGION", "ap-northeast-2"),

		// Worker Configuration
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 20),
		MaxRetries:        getEnvInt("MAX_RETRIES", 5),
		BackoffBaseMS:     getEnvInt("BACKOFF_BASE_MS", 1000),

		// External Services
		InventoryGRPCAddr:  getEnv("INVENTORY_GRPC_ADDR", "inventory-svc:8021"),
		ReservationAPIBase: getEnv("RESERVATION_API_BASE", "http://reservation-api:8010"),

		// Observability
		OTELExporterEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),

		// Server Configuration
		ServerPort:    getEnv("SERVER_PORT", "8040"),
		GRPCDebugPort: getEnv("GRPC_DEBUG_PORT", "8041"),
	}
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as integer with default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets environment variable as boolean with default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// GetBackoffDuration returns the backoff duration for the given attempt
func (c *Config) GetBackoffDuration(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s (max)
	multiplier := 1
	for i := 0; i < attempt && i < 4; i++ {
		multiplier *= 2
	}
	return time.Duration(c.BackoffBaseMS*multiplier) * time.Millisecond
}