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
)

// ReservationClient wraps HTTP client for reservation API
type ReservationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewReservationClient creates a new reservation API client
func NewReservationClient(baseURL string) *ReservationClient {
	return &ReservationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

// UpdateReservationStatus updates the status of a reservation
func (c *ReservationClient) UpdateReservationStatus(ctx context.Context, req *UpdateStatusRequest) error {
	url := fmt.Sprintf("%s/internal/reservations/%s", c.baseURL, req.ReservationID)

	payload := map[string]interface{}{
		"status": req.Status,
	}

	if req.OrderID != "" {
		payload["order_id"] = req.OrderID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetReservation retrieves reservation details
func (c *ReservationClient) GetReservation(ctx context.Context, reservationID string) (*ReservationDetails, error) {
	url := fmt.Sprintf("%s/internal/reservations/%s", c.baseURL, reservationID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var details ReservationDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &details, nil
}

// UpdateStatusRequest represents a request to update reservation status
type UpdateStatusRequest struct {
	ReservationID string
	Status        string // CONFIRMED, CANCELLED, EXPIRED
	OrderID       string // Optional, for CONFIRMED status
}

// ReservationDetails represents reservation information
type ReservationDetails struct {
	ID            string    `json:"reservation_id"`
	EventID       string    `json:"event_id"`
	UserID        string    `json:"user_id"`
	Status        string    `json:"status"`
	SeatIDs       []string  `json:"seat_ids"`
	Quantity      int       `json:"quantity"`
	TotalPrice    int64     `json:"total_price"`
	HoldExpiresAt time.Time `json:"hold_expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Reservation status constants
const (
	StatusHold      = "HOLD"
	StatusConfirmed = "CONFIRMED"
	StatusCancelled = "CANCELLED"
	StatusExpired   = "EXPIRED"
)