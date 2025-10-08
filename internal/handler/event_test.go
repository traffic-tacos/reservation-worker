package handler_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/traffic-tacos/reservation-worker/internal/handler"
)

func TestEvent_ParseEventDetail(t *testing.T) {
	tests := []struct {
		name    string
		event   handler.Event
		want    interface{}
		wantErr bool
	}{
		{
			name: "parse reservation expired event",
			event: handler.Event{
				ID:     "evt_123",
				Type:   handler.EventTypeReservationExpired,
				Source: "reservation-api",
				Time:   time.Now(),
				Detail: json.RawMessage(`{
					"event_id": "evt_123",
					"reservation_id": "rsv_456",
					"qty": 2,
					"seat_ids": ["A1", "A2"]
				}`),
			},
			wantErr: false,
		},
		{
			name: "parse payment approved event",
			event: handler.Event{
				ID:     "evt_789",
				Type:   handler.EventTypePaymentApproved,
				Source: "payment-sim-api",
				Time:   time.Now(),
				Detail: json.RawMessage(`{
					"reservation_id": "rsv_101",
					"payment_intent_id": "pay_abc",
					"amount": 120000
				}`),
			},
			wantErr: false,
		},
		{
			name: "parse payment failed event",
			event: handler.Event{
				ID:     "evt_999",
				Type:   handler.EventTypePaymentFailed,
				Source: "payment-sim-api",
				Time:   time.Now(),
				Detail: json.RawMessage(`{
					"reservation_id": "rsv_222",
					"payment_intent_id": "pay_def",
					"amount": 120000,
					"error_code": "insufficient_funds"
				}`),
			},
			wantErr: false,
		},
		{
			name: "unknown event type returns error",
			event: handler.Event{
				Type:   "unknown.event",
				Detail: json.RawMessage(`{"test": "data"}`),
			},
			wantErr: true,
		},
		{
			name: "invalid payload JSON",
			event: handler.Event{
				Type:   handler.EventTypeReservationExpired,
				Detail: json.RawMessage(`{invalid json}`),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.event.ParseEventDetail()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.ParseEventDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("Event.ParseEventDetail() returned nil, want non-nil")
			}
		})
	}
}

func TestValidateEventType(t *testing.T) {
	tests := []struct {
		eventType string
		want      bool
	}{
		{handler.EventTypeReservationExpired, true},
		{handler.EventTypeReservationHoldExpired, true},
		{handler.EventTypePaymentApproved, true},
		{handler.EventTypePaymentFailed, true},
		{"invalid.event", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			// Create event with the test type and valid JSON detail
			event := handler.Event{
				Type:   tt.eventType,
				Detail: json.RawMessage(`{"test": "data"}`),
			}

			// Check if parsing returns error for invalid types
			_, err := event.ParseEventDetail()
			isValid := err == nil

			if isValid != tt.want {
				t.Errorf("Event type %q validation = %v, want %v (error: %v)", tt.eventType, isValid, tt.want, err)
			}
		})
	}
}
