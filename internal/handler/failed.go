package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/traffic-tacos/proto-contracts/gen/go/reservation/v1"
	"github.com/traffic-tacos/reservation-worker/internal/client"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// FailedHandler handles payment.failed events
type FailedHandler struct {
	inventoryClient   *client.InventoryClient
	reservationClient *client.ReservationClient
	logger            *observability.Logger
	metrics           *observability.Metrics
}

// NewFailedHandler creates a new failed event handler
func NewFailedHandler(
	inventoryClient *client.InventoryClient,
	reservationClient *client.ReservationClient,
	logger *observability.Logger,
	metrics *observability.Metrics,
) *FailedHandler {
	return &FailedHandler{
		inventoryClient:   inventoryClient,
		reservationClient: reservationClient,
		logger:            logger,
		metrics:           metrics,
	}
}

// Handle processes a payment failed event
func (h *FailedHandler) Handle(ctx context.Context, event *Event) error {
	start := time.Now()

	// Parse event detail
	detail, err := event.ParseEventDetail()
	if err != nil {
		h.metrics.RecordProcessingDuration("failed", observability.OutcomeInvalidPayload, time.Since(start).Seconds())
		return fmt.Errorf("failed to parse event detail: %w", err)
	}

	failedDetail, ok := detail.(*PaymentFailedDetail)
	if !ok {
		h.metrics.RecordProcessingDuration("failed", observability.OutcomeInvalidPayload, time.Since(start).Seconds())
		return fmt.Errorf("invalid event detail type for failed event")
	}

	// Start tracing span
	ctx, span := observability.StartSpan(ctx, "handle_payment_failed")
	span.SetAttributes(
		attribute.String("reservation_id", failedDetail.ReservationID),
		attribute.String("payment_intent_id", failedDetail.PaymentIntentID),
		attribute.Int64("amount", failedDetail.Amount),
		attribute.String("error_code", failedDetail.ErrorCode),
	)
	defer span.End()

	logger := h.logger.WithEvent(event.Type, failedDetail.ReservationID, failedDetail.EventID)
	if event.TraceID != "" {
		logger = h.logger.WithTrace(event.TraceID)
	}

	logger.Info("Processing payment failed event",
		zap.String("reservation_id", failedDetail.ReservationID),
		zap.String("payment_intent_id", failedDetail.PaymentIntentID),
		zap.Int64("amount", failedDetail.Amount),
		zap.String("error_code", failedDetail.ErrorCode),
		zap.String("error_message", failedDetail.ErrorMessage),
	)

	// Step 1: Update reservation status to CANCELLED
	statusReq := &client.UpdateStatusRequest{
		ReservationID: failedDetail.ReservationID,
		Status:        client.StatusCancelled,
	}

	if err := h.reservationClient.UpdateReservationStatus(ctx, statusReq); err != nil {
		observability.SetSpanError(span, err)
		h.metrics.RecordProcessingDuration("failed", observability.OutcomeDownstreamError, time.Since(start).Seconds())
		logger.Error("Failed to update reservation status",
			zap.Error(err),
			zap.String("reservation_id", failedDetail.ReservationID),
		)
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	logger.Info("Successfully updated reservation status to CANCELLED",
		zap.String("reservation_id", failedDetail.ReservationID),
	)

	// Step 2: Release hold in inventory service
	if failedDetail.EventID != "" && len(failedDetail.SeatIDs) > 0 {
		releaseReq := &reservationv1.ReleaseHoldRequest{
			EventId:       failedDetail.EventID,
			ReservationId: failedDetail.ReservationID,
			Quantity:      int32(failedDetail.Quantity),
			SeatIds:       failedDetail.SeatIDs,
		}

		if err := h.inventoryClient.ReleaseHold(ctx, releaseReq); err != nil {
			observability.SetSpanError(span, err)
			h.metrics.RecordProcessingDuration("failed", observability.OutcomeDownstreamError, time.Since(start).Seconds())
			logger.Error("Failed to release hold in inventory service",
				zap.Error(err),
				zap.String("reservation_id", failedDetail.ReservationID),
			)
			return fmt.Errorf("failed to release hold: %w", err)
		}

		logger.Info("Successfully released hold in inventory service",
			zap.String("reservation_id", failedDetail.ReservationID),
		)
	}

	// Success
	observability.SetSpanSuccess(span)
	duration := time.Since(start)
	h.metrics.RecordProcessingDuration("failed", observability.OutcomeSuccess, duration.Seconds())

	logger.Info("Successfully processed payment failed event",
		zap.String("reservation_id", failedDetail.ReservationID),
		zap.String("payment_intent_id", failedDetail.PaymentIntentID),
		zap.Duration("duration", duration),
	)

	return nil
}