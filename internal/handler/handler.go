package handler

import (
	"context"
	"fmt"

	"github.com/traffic-tacos/reservation-worker/internal/client"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"github.com/traffic-tacos/reservation-worker/pkg/types"
)

// EventHandler defines the interface for handling events
type EventHandler interface {
	Handle(ctx context.Context, event *types.Event) error
}

// EventHandlerImpl implements EventHandler
type EventHandlerImpl struct {
	inventoryClient   *client.InventoryServiceClient
	reservationClient *client.ReservationServiceClient
	logger            *observability.Logger
	metrics           *observability.Metrics
}

// NewEventHandler creates a new event handler
func NewEventHandler(
	inventoryClient *client.InventoryServiceClient,
	reservationClient *client.ReservationServiceClient,
	logger *observability.Logger,
	metrics *observability.Metrics,
) *EventHandlerImpl {
	return &EventHandlerImpl{
		inventoryClient:   inventoryClient,
		reservationClient: reservationClient,
		logger:            logger,
		metrics:           metrics,
	}
}

// Handle processes an event based on its type
func (h *EventHandlerImpl) Handle(ctx context.Context, event *types.Event) error {
	logger := h.logger.EventLog(ctx, event.Type.String(), event.ReservationID, map[string]interface{}{
		"event_id": event.EventID,
		"attempt":  1, // This would be passed from the worker
	})

	logger.Info("Starting event processing")

	var err error
	switch event.Type {
	case types.EventTypeReservationExpired:
		err = h.handleReservationExpired(ctx, event)
	case types.EventTypePaymentApproved:
		err = h.handlePaymentApproved(ctx, event)
	case types.EventTypePaymentFailed:
		err = h.handlePaymentFailed(ctx, event)
	default:
		err = fmt.Errorf("unknown event type: %s", event.Type)
	}

	if err != nil {
		logger.ErrorLog(ctx, err, nil).Error("Event processing failed")
		h.metrics.RecordEvent(event.Type.String(), observability.OutcomeFailed.String())
		return err
	}

	logger.Info("Event processing completed successfully")
	h.metrics.RecordEvent(event.Type.String(), observability.OutcomeSuccess.String())
	return nil
}

// handleReservationExpired handles reservation.expired events
func (h *EventHandlerImpl) handleReservationExpired(ctx context.Context, event *types.Event) error {
	payload, err := event.GetReservationExpiredPayload()
	if err != nil {
		return fmt.Errorf("failed to parse reservation expired payload: %w", err)
	}

	// Step 1: Update reservation status to EXPIRED
	if err := h.reservationClient.UpdateReservationStatus(ctx, event.ReservationID, client.StatusExpired); err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	// Step 2: Release hold on inventory
	if err := h.inventoryClient.ReleaseHold(ctx, event.EventID, event.ReservationID, payload.Quantity, payload.SeatIDs); err != nil {
		return fmt.Errorf("failed to release inventory hold: %w", err)
	}

	h.logger.EventLog(ctx, event.Type.String(), event.ReservationID, map[string]interface{}{
		"event_id":   event.EventID,
		"quantity":   payload.Quantity,
		"seat_count": len(payload.SeatIDs),
		"action":     "expired_and_released",
	}).Info("Reservation expired and inventory released")

	return nil
}

// handlePaymentApproved handles payment.approved events
func (h *EventHandlerImpl) handlePaymentApproved(ctx context.Context, event *types.Event) error {
	payload, err := event.GetPaymentApprovedPayload()
	if err != nil {
		return fmt.Errorf("failed to parse payment approved payload: %w", err)
	}

	// Step 1: Update reservation status to CONFIRMED
	if err := h.reservationClient.UpdateReservationStatus(ctx, event.ReservationID, client.StatusConfirmed); err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	// Step 2: Commit reservation in inventory (optional - depending on business logic)
	// Note: According to the requirements, this step is optional and may not be needed
	// if the inventory is already committed during the reservation process

	h.logger.EventLog(ctx, event.Type.String(), event.ReservationID, map[string]interface{}{
		"event_id":          event.EventID,
		"payment_intent_id": payload.PaymentIntentID,
		"amount":            payload.Amount,
		"action":            "confirmed",
	}).Info("Payment approved and reservation confirmed")

	return nil
}

// handlePaymentFailed handles payment.failed events
func (h *EventHandlerImpl) handlePaymentFailed(ctx context.Context, event *types.Event) error {
	payload, err := event.GetPaymentFailedPayload()
	if err != nil {
		return fmt.Errorf("failed to parse payment failed payload: %w", err)
	}

	// Step 1: Update reservation status to CANCELLED
	if err := h.reservationClient.UpdateReservationStatus(ctx, event.ReservationID, client.StatusCancelled); err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	// Step 2: Release hold on inventory (since payment failed)
	// Note: We need to get the quantity and seat IDs from somewhere
	// In a real implementation, this information might be stored in the reservation
	// or passed in the event payload. For now, we'll assume it's not available
	// and just update the status.

	h.logger.EventLog(ctx, event.Type.String(), event.ReservationID, map[string]interface{}{
		"event_id":          event.EventID,
		"payment_intent_id": payload.PaymentIntentID,
		"amount":            payload.Amount,
		"action":            "cancelled",
	}).Info("Payment failed and reservation cancelled")

	return nil
}

