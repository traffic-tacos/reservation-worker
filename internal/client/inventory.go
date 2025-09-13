package client

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/traffic-tacos/reservation-worker/internal/observability"
)

// InventoryServiceClient wraps the gRPC client for inventory service
type InventoryServiceClient struct {
	conn   *grpc.ClientConn
	client InventoryClient // This would be generated from protobuf
	logger *observability.Logger
}

// NewInventoryServiceClient creates a new inventory service client
func NewInventoryServiceClient(addr string, logger *observability.Logger) (*InventoryServiceClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// TODO: Add OpenTelemetry interceptors when available
		// grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		// grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inventory service: %w", err)
	}

	// In a real implementation, this would be:
	// client := pb.NewInventoryClient(conn)
	// For now, we'll create a mock client
	client := &mockInventoryClient{}

	return &InventoryServiceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}, nil
}

// Close closes the gRPC connection
func (c *InventoryServiceClient) Close() error {
	return c.conn.Close()
}

// ReleaseHold releases a hold on inventory for expired reservations
func (c *InventoryServiceClient) ReleaseHold(ctx context.Context, eventID, reservationID string, qty int, seatIDs []string) error {
	req := &ReleaseReq{
		EventId:       eventID,
		ReservationId: reservationID,
		Qty:           int32(qty),
		SeatIds:       seatIDs,
	}

	resp, err := c.client.ReleaseHold(ctx, req)
	if err != nil {
		// Convert gRPC status to error
		if grpcStatus, ok := status.FromError(err); ok {
			return fmt.Errorf("inventory release hold failed with code %s: %s", grpcStatus.Code(), grpcStatus.Message())
		}
		return fmt.Errorf("inventory release hold failed: %w", err)
	}

	if resp.Status != "OK" {
		return fmt.Errorf("inventory release hold returned status: %s", resp.Status)
	}

	c.logger.WithContext(ctx).Logger.With(
		zap.String("event_id", eventID),
		zap.String("reservation_id", reservationID),
		zap.Int("quantity", qty),
		zap.Int("seat_count", len(seatIDs)),
	).Info("Successfully released hold on inventory")

	return nil
}

// CommitReservation commits a reservation to sold status
func (c *InventoryServiceClient) CommitReservation(ctx context.Context, reservationID, eventID string, qty int, seatIDs []string, paymentIntentID string) error {
	req := &CommitReq{
		ReservationId:   reservationID,
		EventId:         eventID,
		Qty:             int32(qty),
		SeatIds:         seatIDs,
		PaymentIntentId: paymentIntentID,
	}

	resp, err := c.client.CommitReservation(ctx, req)
	if err != nil {
		// Convert gRPC status to error
		if grpcStatus, ok := status.FromError(err); ok {
			return fmt.Errorf("inventory commit reservation failed with code %s: %s", grpcStatus.Code(), grpcStatus.Message())
		}
		return fmt.Errorf("inventory commit reservation failed: %w", err)
	}

	if resp.Status != "COMMITTED" {
		return fmt.Errorf("inventory commit reservation returned status: %s", resp.Status)
	}

	c.logger.WithContext(ctx).Logger.Info("Successfully committed reservation",
		zap.String("reservation_id", reservationID),
		zap.String("event_id", eventID),
		zap.Int("quantity", qty),
		zap.Int("seat_count", len(seatIDs)),
		zap.String("payment_intent_id", paymentIntentID),
	)

	return nil
}

// Mock implementations for development (would be replaced by generated code)

// InventoryClient interface (would be generated from protobuf)
type InventoryClient interface {
	ReleaseHold(ctx context.Context, req *ReleaseReq) (*ReleaseRes, error)
	CommitReservation(ctx context.Context, req *CommitReq) (*CommitRes, error)
}

// ReleaseReq represents the release hold request
type ReleaseReq struct {
	EventId       string   `json:"event_id"`
	ReservationId string   `json:"reservation_id"`
	Qty           int32    `json:"qty"`
	SeatIds       []string `json:"seat_ids"`
}

// ReleaseRes represents the release hold response
type ReleaseRes struct {
	Status string `json:"status"`
}

// CommitReq represents the commit reservation request
type CommitReq struct {
	ReservationId   string   `json:"reservation_id"`
	EventId         string   `json:"event_id"`
	Qty             int32    `json:"qty"`
	SeatIds         []string `json:"seat_ids"`
	PaymentIntentId string   `json:"payment_intent_id"`
}

// CommitRes represents the commit reservation response
type CommitRes struct {
	OrderId string `json:"order_id"`
	Status  string `json:"status"`
}

// mockInventoryClient is a mock implementation for development
type mockInventoryClient struct{}

func (m *mockInventoryClient) ReleaseHold(ctx context.Context, req *ReleaseReq) (*ReleaseRes, error) {
	// Simulate successful release
	return &ReleaseRes{Status: "OK"}, nil
}

func (m *mockInventoryClient) CommitReservation(ctx context.Context, req *CommitReq) (*CommitRes, error) {
	// Simulate successful commit
	return &CommitRes{
		OrderId: "ord_" + req.ReservationId[4:], // Simple transformation for mock
		Status:  "COMMITTED",
	}, nil
}
