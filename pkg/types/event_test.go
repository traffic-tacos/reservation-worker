package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_String(t *testing.T) {
	assert.Equal(t, "reservation.expired", EventTypeReservationExpired.String())
	assert.Equal(t, "payment.approved", EventTypePaymentApproved.String())
	assert.Equal(t, "payment.failed", EventTypePaymentFailed.String())
}

func TestEvent_GetReservationExpiredPayload(t *testing.T) {
	event := &Event{
		Payload: map[string]interface{}{
			"qty":      2.0,
			"seat_ids": []interface{}{"A1", "A2"},
		},
	}

	payload, err := event.GetReservationExpiredPayload()
	require.NoError(t, err)
	assert.Equal(t, 2, payload.Quantity)
	assert.Equal(t, []string{"A1", "A2"}, payload.SeatIDs)
}

func TestEvent_GetPaymentApprovedPayload(t *testing.T) {
	event := &Event{
		Payload: map[string]interface{}{
			"payment_intent_id": "pay_123",
			"amount":            1000.0,
		},
	}

	payload, err := event.GetPaymentApprovedPayload()
	require.NoError(t, err)
	assert.Equal(t, "pay_123", payload.PaymentIntentID)
	assert.Equal(t, 1000.0, payload.Amount)
}

func TestEvent_GetPaymentFailedPayload(t *testing.T) {
	event := &Event{
		Payload: map[string]interface{}{
			"payment_intent_id": "pay_456",
			"amount":            2000.0,
		},
	}

	payload, err := event.GetPaymentFailedPayload()
	require.NoError(t, err)
	assert.Equal(t, "pay_456", payload.PaymentIntentID)
	assert.Equal(t, 2000.0, payload.Amount)
}

func TestEvent_PayloadParsingErrors(t *testing.T) {
	// Test with missing qty field
	event := &Event{
		Payload: map[string]interface{}{
			"seat_ids": []interface{}{"A1"},
		},
	}

	payload, err := event.GetReservationExpiredPayload()
	require.NoError(t, err) // Should not error, qty defaults to 0
	assert.Equal(t, 0, payload.Quantity)
	assert.Equal(t, []string{"A1"}, payload.SeatIDs)
}

func TestEventType(t *testing.T) {
	event := &Event{
		ID:            "evt_123",
		Type:          EventTypeReservationExpired,
		ReservationID: "rsv_456",
		EventID:       "evt_789",
		Timestamp:     time.Now(),
		Payload: map[string]interface{}{
			"qty":      2.0,
			"seat_ids": []interface{}{"A1", "A2"},
		},
		TraceID: "trace_123",
	}

	assert.Equal(t, "evt_123", event.ID)
	assert.Equal(t, EventTypeReservationExpired, event.Type)
	assert.Equal(t, "rsv_456", event.ReservationID)
	assert.Equal(t, "evt_789", event.EventID)
	assert.NotZero(t, event.Timestamp)
	assert.Equal(t, "trace_123", event.TraceID)
}

