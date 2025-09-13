package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Set up environment variables
	envVars := map[string]string{
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

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	// Test loading configuration
	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify configuration values
	assert.Equal(t, "https://sqs.ap-northeast-2.amazonaws.com/123/queue", cfg.SQSQueueURL)
	assert.Equal(t, 20, cfg.SQSWaitTime)
	assert.Equal(t, 20, cfg.WorkerConcurrency)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 1000, cfg.BackoffBaseMs)
	assert.Equal(t, time.Duration(1000)*time.Millisecond, cfg.BackoffBaseDuration)
	assert.Equal(t, "inventory-svc:8080", cfg.InventoryGRPCAddr)
	assert.Equal(t, "http://reservation-api:8080", cfg.ReservationAPIBase)
	assert.Equal(t, "http://otel-collector:4317", cfg.OtelExporterOTLPEndpoint)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_MissingRequired(t *testing.T) {
	// Clear all environment variables
	for key := range GetEnvMap() {
		os.Unsetenv(key)
	}

	// Test loading configuration with missing required vars
	cfg, err := Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				SQSQueueURL:        "https://sqs.ap-northeast-2.amazonaws.com/123/queue",
				SQSWaitTime:        20,
				WorkerConcurrency:  20,
				MaxRetries:         5,
				BackoffBaseMs:      1000,
				InventoryGRPCAddr:  "inventory-svc:8080",
				ReservationAPIBase: "http://reservation-api:8080",
				LogLevel:           "info",
			},
			expectError: false,
		},
		{
			name: "invalid sqs wait time - too high",
			config: &Config{
				SQSQueueURL:        "https://sqs.ap-northeast-2.amazonaws.com/123/queue",
				SQSWaitTime:        25, // > 20
				WorkerConcurrency:  20,
				MaxRetries:         5,
				BackoffBaseMs:      1000,
				InventoryGRPCAddr:  "inventory-svc:8080",
				ReservationAPIBase: "http://reservation-api:8080",
				LogLevel:           "info",
			},
			expectError: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				SQSQueueURL:        "https://sqs.ap-northeast-2.amazonaws.com/123/queue",
				SQSWaitTime:        20,
				WorkerConcurrency:  20,
				MaxRetries:         5,
				BackoffBaseMs:      1000,
				InventoryGRPCAddr:  "inventory-svc:8080",
				ReservationAPIBase: "http://reservation-api:8080",
				LogLevel:           "invalid",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_GetBackoffDuration(t *testing.T) {
	cfg := &Config{
		BackoffBaseDuration: time.Second,
	}

	// Test backoff calculation
	assert.Equal(t, time.Second, cfg.GetBackoffDuration(0))
	assert.Equal(t, 2*time.Second, cfg.GetBackoffDuration(1))
	assert.Equal(t, 4*time.Second, cfg.GetBackoffDuration(2))
	assert.Equal(t, 8*time.Second, cfg.GetBackoffDuration(3))
}

func TestConfig_IsMaxRetriesReached(t *testing.T) {
	cfg := &Config{MaxRetries: 3}

	assert.False(t, cfg.IsMaxRetriesReached(0))
	assert.False(t, cfg.IsMaxRetriesReached(2))
	assert.True(t, cfg.IsMaxRetriesReached(3))
	assert.True(t, cfg.IsMaxRetriesReached(4))
}

func TestGetEnvMap(t *testing.T) {
	envMap := GetEnvMap()
	assert.NotEmpty(t, envMap)

	// Check that all expected keys are present
	expectedKeys := []string{
		"SQS_QUEUE_URL",
		"SQS_WAIT_TIME",
		"WORKER_CONCURRENCY",
		"MAX_RETRIES",
		"BACKOFF_BASE_MS",
		"INVENTORY_GRPC_ADDR",
		"RESERVATION_API_BASE",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"LOG_LEVEL",
	}

	for _, key := range expectedKeys {
		assert.Contains(t, envMap, key)
		assert.NotEmpty(t, envMap[key])
	}
}
