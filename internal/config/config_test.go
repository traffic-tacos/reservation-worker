package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/traffic-tacos/reservation-worker/internal/config"
)

func TestLoad(t *testing.T) {
	// Set test environment variables
	os.Setenv("SQS_QUEUE_URL", "https://sqs.test.amazonaws.com/test-queue")
	os.Setenv("WORKER_CONCURRENCY", "10")
	os.Setenv("AWS_PROFILE", "test-profile")
	os.Setenv("USE_SECRET_MANAGER", "true")
	defer func() {
		os.Unsetenv("SQS_QUEUE_URL")
		os.Unsetenv("WORKER_CONCURRENCY")
		os.Unsetenv("AWS_PROFILE")
		os.Unsetenv("USE_SECRET_MANAGER")
	}()

	cfg := config.Load()

	// Test loaded values
	if cfg.SQSQueueURL != "https://sqs.test.amazonaws.com/test-queue" {
		t.Errorf("Expected SQSQueueURL to be 'https://sqs.test.amazonaws.com/test-queue', got '%s'", cfg.SQSQueueURL)
	}

	if cfg.WorkerConcurrency != 10 {
		t.Errorf("Expected WorkerConcurrency to be 10, got %d", cfg.WorkerConcurrency)
	}

	if cfg.AWSProfile != "test-profile" {
		t.Errorf("Expected AWSProfile to be 'test-profile', got '%s'", cfg.AWSProfile)
	}

	if !cfg.UseSecretManager {
		t.Error("Expected UseSecretManager to be true")
	}

	// Test default values
	if cfg.MaxRetries != 5 {
		t.Errorf("Expected default MaxRetries to be 5, got %d", cfg.MaxRetries)
	}

	if cfg.BackoffBaseMS != 1000 {
		t.Errorf("Expected default BackoffBaseMS to be 1000, got %d", cfg.BackoffBaseMS)
	}
}

func TestGetBackoffDuration(t *testing.T) {
	cfg := &config.Config{
		BackoffBaseMS: 1000, // 1 second base
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},   // 1s
		{1, 2 * time.Second},   // 2s
		{2, 4 * time.Second},   // 4s
		{3, 8 * time.Second},   // 8s
		{4, 16 * time.Second},  // 16s (max)
		{5, 16 * time.Second},  // 16s (capped at max)
		{10, 16 * time.Second}, // 16s (capped at max)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := cfg.GetBackoffDuration(tt.attempt)
			if got != tt.expected {
				t.Errorf("GetBackoffDuration(%d) = %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestLoadWithDefaults(t *testing.T) {
	// Clear all relevant environment variables to test defaults
	envVars := []string{
		"AWS_PROFILE",
		"AWS_REGION",
		"USE_SECRET_MANAGER",
		"SQS_QUEUE_URL",
		"SQS_WAIT_TIME",
		"WORKER_CONCURRENCY",
		"MAX_RETRIES",
		"INVENTORY_GRPC_ADDR",
		"RESERVATION_API_BASE",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"LOG_LEVEL",
		"SERVER_PORT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	cfg := config.Load()

	// Check default values
	if cfg.AWSProfile != "tacos" {
		t.Errorf("Expected default AWSProfile to be 'tacos', got '%s'", cfg.AWSProfile)
	}

	if cfg.AWSRegion != "ap-northeast-2" {
		t.Errorf("Expected default AWSRegion to be 'ap-northeast-2', got '%s'", cfg.AWSRegion)
	}

	if cfg.UseSecretManager {
		t.Error("Expected default UseSecretManager to be false")
	}

	if cfg.SQSWaitTime != 20 {
		t.Errorf("Expected default SQSWaitTime to be 20, got %d", cfg.SQSWaitTime)
	}

	if cfg.WorkerConcurrency != 20 {
		t.Errorf("Expected default WorkerConcurrency to be 20, got %d", cfg.WorkerConcurrency)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel to be 'info', got '%s'", cfg.LogLevel)
	}

	if cfg.ServerPort != "8040" {
		t.Errorf("Expected default ServerPort to be '8040', got '%s'", cfg.ServerPort)
	}
}