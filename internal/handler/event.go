package handler

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event represents a reservation/payment event from SQS
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Source    string          `json:"source"`
	Detail    json.RawMessage `json:"detail"`
	Time      time.Time       `json:"time"`
	TraceID   string          `json:"trace_id,omitempty"`
	Version   string          `json:"version,omitempty"`
	Region    string          `json:"region,omitempty"`
	Account   string          `json:"account,omitempty"`
	Resources []string        `json:"resources,omitempty"`
}

// ReservationExpiredDetail represents the detail for reservation.expired events
type ReservationExpiredDetail struct {
	ReservationID string   `json:"reservation_id"`
	EventID       string   `json:"event_id"`
	Quantity      int      `json:"qty"`
	SeatIDs       []string `json:"seat_ids"`
	UserID        string   `json:"user_id,omitempty"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
}

// PaymentApprovedDetail represents the detail for payment.approved events
type PaymentApprovedDetail struct {
	ReservationID   string   `json:"reservation_id"`
	PaymentIntentID string   `json:"payment_intent_id"`
	Amount          int64    `json:"amount"`
	Currency        string   `json:"currency,omitempty"`
	EventID         string   `json:"event_id,omitempty"`
	UserID          string   `json:"user_id,omitempty"`
	SeatIDs         []string `json:"seat_ids,omitempty"`
	Quantity        int      `json:"qty,omitempty"`
}

// PaymentFailedDetail represents the detail for payment.failed events
type PaymentFailedDetail struct {
	ReservationID   string   `json:"reservation_id"`
	PaymentIntentID string   `json:"payment_intent_id"`
	Amount          int64    `json:"amount"`
	Currency        string   `json:"currency,omitempty"`
	ErrorCode       string   `json:"error_code,omitempty"`
	ErrorMessage    string   `json:"error_message,omitempty"`
	EventID         string   `json:"event_id,omitempty"`
	UserID          string   `json:"user_id,omitempty"`
	SeatIDs         []string `json:"seat_ids,omitempty"`
	Quantity        int      `json:"qty,omitempty"`
}

// Event type constants
const (
	EventTypeReservationExpired = "reservation.expired"
	EventTypePaymentApproved    = "payment.approved"
	EventTypePaymentFailed      = "payment.failed"

	// Legacy event types for compatibility
	EventTypeReservationHoldCreated = "reservation.hold.created"
	EventTypeReservationHoldExpired = "reservation.hold.expired"
)

// ParseEventDetail parses the event detail based on event type
func (e *Event) ParseEventDetail() (interface{}, error) {
	switch e.Type {
	case EventTypeReservationExpired, EventTypeReservationHoldExpired:
		var detail ReservationExpiredDetail
		if err := json.Unmarshal(e.Detail, &detail); err != nil {
			return nil, err
		}
		return &detail, nil

	case EventTypePaymentApproved:
		var detail PaymentApprovedDetail
		if err := json.Unmarshal(e.Detail, &detail); err != nil {
			return nil, err
		}
		return &detail, nil

	case EventTypePaymentFailed:
		var detail PaymentFailedDetail
		if err := json.Unmarshal(e.Detail, &detail); err != nil {
			return nil, err
		}
		return &detail, nil

	default:
		// Return error for unknown event types
		return nil, fmt.Errorf("unknown event type: %s", e.Type)
	}
}
