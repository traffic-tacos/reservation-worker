package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretConfig represents the structure of secrets stored in AWS Secrets Manager
type SecretConfig struct {
	SQSQueueURL        string `json:"sqs_queue_url"`
	InventoryGRPCAddr  string `json:"inventory_grpc_addr"`
	ReservationAPIBase string `json:"reservation_api_base"`
	OTELEndpoint       string `json:"otel_endpoint"`
}

// LoadSecretsFromAWS loads configuration from AWS Secrets Manager
func LoadSecretsFromAWS(ctx context.Context, region, secretName, profile string) (*SecretConfig, error) {
	// Create AWS config
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	// Use profile if specified
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Secrets Manager client
	client := secretsmanager.NewFromConfig(cfg)

	// Get secret value
	result, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret value: %w", err)
	}

	// Parse secret value as JSON
	var secretConfig SecretConfig
	if result.SecretString != nil {
		if err := json.Unmarshal([]byte(*result.SecretString), &secretConfig); err != nil {
			return nil, fmt.Errorf("failed to parse secret value: %w", err)
		}
	}

	return &secretConfig, nil
}

// MergeWithSecrets merges configuration with secrets from AWS Secrets Manager
func (c *Config) MergeWithSecrets(ctx context.Context) error {
	if !c.UseSecretManager {
		return nil
	}

	secrets, err := LoadSecretsFromAWS(ctx, c.AWSRegion, c.SecretName, c.AWSProfile)
	if err != nil {
		return fmt.Errorf("failed to load secrets: %w", err)
	}

	// Override config with secrets if they are not empty
	if secrets.SQSQueueURL != "" {
		c.SQSQueueURL = secrets.SQSQueueURL
	}
	if secrets.InventoryGRPCAddr != "" {
		c.InventoryGRPCAddr = secrets.InventoryGRPCAddr
	}
	if secrets.ReservationAPIBase != "" {
		c.ReservationAPIBase = secrets.ReservationAPIBase
	}
	if secrets.OTELEndpoint != "" {
		c.OTELExporterEndpoint = secrets.OTELEndpoint
	}

	return nil
}