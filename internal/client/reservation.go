package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"

	"github.com/traffic-tacos/reservation-worker/internal/observability"
)

// ReservationServiceClient wraps the HTTP client for reservation service
type ReservationServiceClient struct {
	baseURL string
	client  *http.Client
	logger  *observability.Logger
}

// NewReservationServiceClient creates a new reservation service client
func NewReservationServiceClient(baseURL string, logger *observability.Logger) *ReservationServiceClient {
	// Create HTTP client with OpenTelemetry instrumentation
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	return &ReservationServiceClient{
		baseURL: baseURL,
		client:  client,
		logger:  logger,
	}
}

// ReservationStatus represents the status of a reservation
type ReservationStatus string

const (
	StatusHold      ReservationStatus = "HOLD"
	StatusConfirmed ReservationStatus = "CONFIRMED"
	StatusCancelled ReservationStatus = "CANCELLED"
	StatusExpired   ReservationStatus = "EXPIRED"
)

// String returns the string representation of ReservationStatus
func (s ReservationStatus) String() string {
	return string(s)
}

// UpdateReservationStatus updates the status of a reservation
func (c *ReservationServiceClient) UpdateReservationStatus(ctx context.Context, reservationID string, status ReservationStatus) error {
	url := fmt.Sprintf("%s/internal/reservations/%s", c.baseURL, reservationID)

	reqBody := map[string]interface{}{
		"status": status.String(),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("reservation API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		c.logger.WithContext(ctx).Logger.Warn("Failed to unmarshal response body",
			zap.String("reservation_id", reservationID),
			zap.String("status", string(status)),
			zap.String("response_body", string(body)),
		)
	} else {
		c.logger.WithContext(ctx).Logger.Info("Successfully updated reservation status",
			zap.String("reservation_id", reservationID),
			zap.String("status", string(status)),
			zap.Any("response", response),
		)
	}

	return nil
}

// GetReservationStatus retrieves the current status of a reservation
func (c *ReservationServiceClient) GetReservationStatus(ctx context.Context, reservationID string) (ReservationStatus, error) {
	url := fmt.Sprintf("%s/internal/reservations/%s", c.baseURL, reservationID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("reservation API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	statusInterface, ok := response["status"]
	if !ok {
		return "", fmt.Errorf("status field not found in response")
	}

	statusStr, ok := statusInterface.(string)
	if !ok {
		return "", fmt.Errorf("status field is not a string")
	}

	status := ReservationStatus(statusStr)
	return status, nil
}
