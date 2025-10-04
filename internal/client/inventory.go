package client

import (
	"context"
	"fmt"
	"time"

	"github.com/traffic-tacos/proto-contracts/gen/go/reservation/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InventoryClient wraps gRPC client for inventory service
type InventoryClient struct {
	client reservationv1.InventoryServiceClient
	conn   *grpc.ClientConn
}

// NewInventoryClient creates a new inventory service client
func NewInventoryClient(addr string) (*InventoryClient, error) {
	// Create gRPC connection with OpenTelemetry instrumentation
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to inventory service: %w", err)
	}

	client := reservationv1.NewInventoryServiceClient(conn)

	return &InventoryClient{
		client: client,
		conn:   conn,
	}, nil
}

// Close closes the gRPC connection
func (c *InventoryClient) Close() error {
	return c.conn.Close()
}

// ReleaseHold releases held seats/inventory back to available pool
func (c *InventoryClient) ReleaseHold(ctx context.Context, req *reservationv1.ReleaseHoldRequest) error {
	// Set timeout for gRPC call
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	_, err := c.client.ReleaseHold(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to release hold: %w", err)
	}

	return nil
}

// CommitReservation commits a reservation, marking seats as sold
func (c *InventoryClient) CommitReservation(ctx context.Context, req *reservationv1.CommitReservationRequest) error {
	// Set timeout for gRPC call
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	_, err := c.client.CommitReservation(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to commit reservation: %w", err)
	}

	return nil
}