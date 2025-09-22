package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/traffic-tacos/reservation-worker/internal/client"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// ExpiredHandler handles reservation.expired events
type ExpiredHandler struct {
	inventoryClient   *client.InventoryClient
	reservationClient *client.ReservationClient
	logger            *observability.Logger
	metrics           *observability.Metrics
}

// NewExpiredHandler creates a new expired event handler
func NewExpiredHandler(
	inventoryClient *client.InventoryClient,
	reservationClient *client.ReservationClient,
	logger *observability.Logger,
	metrics *observability.Metrics,
) *ExpiredHandler {
	return &ExpiredHandler{
		inventoryClient:   inventoryClient,
		reservationClient: reservationClient,
		logger:            logger,
		metrics:           metrics,
	}
}

// Handle processes a reservation expired event
func (h *ExpiredHandler) Handle(ctx context.Context, event *Event) error {
	start := time.Now()

	// Parse event detail
	detail, err := event.ParseEventDetail()
	if err != nil {
		h.metrics.RecordProcessingDuration("expired", observability.OutcomeInvalidPayload, time.Since(start).Seconds())
		return fmt.Errorf("failed to parse event detail: %w", err)
	}

	expiredDetail, ok := detail.(*ReservationExpiredDetail)
	if !ok {
		h.metrics.RecordProcessingDuration("expired", observability.OutcomeInvalidPayload, time.Since(start).Seconds())
		return fmt.Errorf("invalid event detail type for expired event")
	}

	// Start tracing span
	ctx, span := observability.StartSpan(ctx, "handle_reservation_expired")
	span.SetAttributes(
		attribute.String("reservation_id", expiredDetail.ReservationID),
		attribute.String("event_id", expiredDetail.EventID),
		attribute.Int("quantity", expiredDetail.Quantity),
	)
	defer span.End()

	logger := h.logger.WithEvent(event.Type, expiredDetail.ReservationID, expiredDetail.EventID)
	if event.TraceID != "" {
		logger = h.logger.WithTrace(event.TraceID)
	}

	logger.Info("Processing reservation expired event",
		zap.String("reservation_id", expiredDetail.ReservationID),
		zap.String("event_id", expiredDetail.EventID),
		zap.Int("quantity", expiredDetail.Quantity),
		zap.Strings("seat_ids", expiredDetail.SeatIDs),
	)

	// Step 1: Release hold in inventory service
	releaseReq := &client.ReleaseHoldRequest{
		EventID:       expiredDetail.EventID,
		ReservationID: expiredDetail.ReservationID,
		Quantity:      expiredDetail.Quantity,
		SeatIDs:       expiredDetail.SeatIDs,
	}

	if err := h.inventoryClient.ReleaseHold(ctx, releaseReq); err != nil {
		observability.SetSpanError(span, err)
		h.metrics.RecordProcessingDuration("expired", observability.OutcomeDownstreamError, time.Since(start).Seconds())
		logger.Error("Failed to release hold in inventory service",
			zap.Error(err),
			zap.String("reservation_id", expiredDetail.ReservationID),
		)
		return fmt.Errorf("failed to release hold: %w", err)
	}

	logger.Info("Successfully released hold in inventory service",
		zap.String("reservation_id", expiredDetail.ReservationID),
	)

	// Step 2: Update reservation status to EXPIRED
	statusReq := &client.UpdateStatusRequest{
		ReservationID: expiredDetail.ReservationID,
		Status:        client.StatusExpired,
	}

	if err := h.reservationClient.UpdateReservationStatus(ctx, statusReq); err != nil {
		observability.SetSpanError(span, err)
		h.metrics.RecordProcessingDuration("expired", observability.OutcomeDownstreamError, time.Since(start).Seconds())
		logger.Error("Failed to update reservation status",
			zap.Error(err),
			zap.String("reservation_id", expiredDetail.ReservationID),
		)
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	// Success
	observability.SetSpanSuccess(span)
	duration := time.Since(start)
	h.metrics.RecordProcessingDuration("expired", observability.OutcomeSuccess, duration.Seconds())

	logger.Info("Successfully processed reservation expired event",
		zap.String("reservation_id", expiredDetail.ReservationID),
		zap.Duration("duration", duration),
	)

	return nil
}