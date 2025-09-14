package types

import "time"

// EventType represents the type of event
type EventType string

const (
	EventTypeReservationExpired EventType = "reservation.expired"
	EventTypePaymentApproved    EventType = "payment.approved"
	EventTypePaymentFailed      EventType = "payment.failed"
)

// String returns the string representation of EventType
func (e EventType) String() string {
	return string(e)
}

// Event represents a reservation worker event
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	ReservationID string                 `json:"reservation_id"`
	EventID       string                 `json:"event_id"`
	Timestamp     time.Time              `json:"ts"`
	Payload       map[string]interface{} `json:"payload"`
	TraceID       string                 `json:"trace_id,omitempty"`
}

// ReservationExpiredPayload represents the payload for reservation.expired events
type ReservationExpiredPayload struct {
	Quantity int      `json:"qty"`
	SeatIDs  []string `json:"seat_ids"`
}

// PaymentApprovedPayload represents the payload for payment.approved events
type PaymentApprovedPayload struct {
	PaymentIntentID string  `json:"payment_intent_id"`
	Amount          float64 `json:"amount"`
}

// PaymentFailedPayload represents the payload for payment.failed events
type PaymentFailedPayload struct {
	PaymentIntentID string  `json:"payment_intent_id"`
	Amount          float64 `json:"amount"`
}

// GetReservationExpiredPayload extracts ReservationExpiredPayload from event payload
func (e *Event) GetReservationExpiredPayload() (*ReservationExpiredPayload, error) {
	payload := &ReservationExpiredPayload{}
	if qty, ok := e.Payload["qty"].(float64); ok {
		payload.Quantity = int(qty)
	}
	if seatIDs, ok := e.Payload["seat_ids"].([]interface{}); ok {
		for _, id := range seatIDs {
			if strID, ok := id.(string); ok {
				payload.SeatIDs = append(payload.SeatIDs, strID)
			}
		}
	}
	return payload, nil
}

// GetPaymentApprovedPayload extracts PaymentApprovedPayload from event payload
func (e *Event) GetPaymentApprovedPayload() (*PaymentApprovedPayload, error) {
	payload := &PaymentApprovedPayload{}
	if paymentIntentID, ok := e.Payload["payment_intent_id"].(string); ok {
		payload.PaymentIntentID = paymentIntentID
	}
	if amount, ok := e.Payload["amount"].(float64); ok {
		payload.Amount = amount
	}
	return payload, nil
}

// GetPaymentFailedPayload extracts PaymentFailedPayload from event payload
func (e *Event) GetPaymentFailedPayload() (*PaymentFailedPayload, error) {
	payload := &PaymentFailedPayload{}
	if paymentIntentID, ok := e.Payload["payment_intent_id"].(string); ok {
		payload.PaymentIntentID = paymentIntentID
	}
	if amount, ok := e.Payload["amount"].(float64); ok {
		payload.Amount = amount
	}
	return payload, nil
}

