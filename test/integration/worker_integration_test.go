//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	eventTypes "github.com/traffic-tacos/reservation-worker/pkg/types"
)

// TestWorkerIntegration tests the worker with LocalStack SQS
func TestWorkerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup LocalStack configuration
	localStackEndpoint := "http://localhost:4566"
	queueName := "test-reservation-queue"

	// Create AWS config for LocalStack
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithEndpointResolver(aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{URL: localStackEndpoint}, nil
			},
		)),
	)
	require.NoError(t, err)

	sqsClient := sqs.NewFromConfig(awsCfg)

	// Create test queue
	createQueueInput := &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
	}
	queueResp, err := sqsClient.CreateQueue(context.Background(), createQueueInput)
	require.NoError(t, err)
	queueURL := queueResp.QueueUrl

	defer func() {
		// Clean up queue
		_, _ = sqsClient.DeleteQueue(context.Background(), &sqs.DeleteQueueInput{
			QueueUrl: queueURL,
		})
	}()

	// Create test configuration
	testCfg := &config.Config{
		SQSQueueURL:       *queueURL,
		SQSWaitTime:       1, // Short wait time for testing
		WorkerConcurrency: 1,
		MaxRetries:        1,
		BackoffBaseMs:     100,
		LogLevel:          "debug",
	}

	// Create test observability
	logger, err := observability.NewLogger(testCfg.LogLevel)
	require.NoError(t, err)

	metrics := observability.NewMetrics()

	// Create mock clients (simplified for integration test)
	inventoryClient := &mockInventoryClient{}
	reservationClient := &mockReservationClient{}

	// Create event handler
	handler := &mockEventHandler{
		inventoryClient:   inventoryClient,
		reservationClient: reservationClient,
		logger:            logger,
		metrics:           metrics,
	}

	// Create worker
	worker, err := NewTestWorker(testCfg, handler, logger, metrics, sqsClient)
	require.NoError(t, err)

	// Start worker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = worker.Start(ctx)
	require.NoError(t, err)

	// Give worker time to start
	time.Sleep(100 * time.Millisecond)

	// Send test message
	testEvent := &eventTypes.Event{
		ID:            "test-event-1",
		Type:          eventTypes.EventTypeReservationExpired,
		ReservationID: "rsv_test_123",
		EventID:       "evt_test_456",
		Timestamp:     time.Now(),
		Payload: map[string]interface{}{
			"qty":      2.0,
			"seat_ids": []interface{}{"A1", "A2"},
		},
	}

	eventJSON, err := json.Marshal(testEvent)
	require.NoError(t, err)

	sendInput := &sqs.SendMessageInput{
		QueueUrl:    queueURL,
		MessageBody: aws.String(string(eventJSON)),
	}

	_, err = sqsClient.SendMessage(context.Background(), sendInput)
	require.NoError(t, err)

	// Wait for processing
	time.Sleep(2 * time.Second)

	// Stop worker
	err = worker.Stop(context.Background())
	require.NoError(t, err)

	// Verify the event was processed
	assert.True(t, handler.eventProcessed)
	assert.Equal(t, eventTypes.EventTypeReservationExpired, handler.processedEventType)
	assert.Equal(t, "rsv_test_123", handler.processedReservationID)
}

// mockEventHandler is a mock implementation for testing
type mockEventHandler struct {
	inventoryClient        *mockInventoryClient
	reservationClient      *mockReservationClient
	logger                 *observability.Logger
	metrics                *observability.Metrics
	eventProcessed         bool
	processedEventType     eventTypes.EventType
	processedReservationID string
}

func (h *mockEventHandler) Handle(ctx context.Context, event *eventTypes.Event) error {
	h.eventProcessed = true
	h.processedEventType = event.Type
	h.processedReservationID = event.ReservationID

	h.logger.EventLog(ctx, event.Type.String(), event.ReservationID, nil).Info("Mock event processed")
	h.metrics.RecordEvent(event.Type.String(), observability.OutcomeSuccess.String())

	return nil
}

// mockInventoryClient is a mock inventory client for testing
type mockInventoryClient struct{}

func (m *mockInventoryClient) ReleaseHold(ctx context.Context, eventID, reservationID string, qty int, seatIDs []string) error {
	return nil
}

func (m *mockInventoryClient) CommitReservation(ctx context.Context, reservationID, eventID string, qty int, seatIDs []string, paymentIntentID string) error {
	return nil
}

func (m *mockInventoryClient) Close() error {
	return nil
}

// mockReservationClient is a mock reservation client for testing
type mockReservationClient struct{}

func (m *mockReservationClient) UpdateReservationStatus(ctx context.Context, reservationID string, status string) error {
	return nil
}

func (m *mockReservationClient) GetReservationStatus(ctx context.Context, reservationID string) (string, error) {
	return "HOLD", nil
}

// TestWorker is a testable version of Worker
type TestWorker struct {
	// Embed the real worker fields we need
	config    *config.Config
	sqsClient *sqs.Client
	queueURL  string
	handler   EventHandler
	logger    *observability.Logger
	metrics   *observability.Metrics
	stopChan  chan struct{}
	stopOnce  sync.Once
}

// EventHandler interface for testing
type EventHandler interface {
	Handle(ctx context.Context, event *eventTypes.Event) error
}

// NewTestWorker creates a testable worker
func NewTestWorker(cfg *config.Config, handler EventHandler, logger *observability.Logger, metrics *observability.Metrics, sqsClient *sqs.Client) (*TestWorker, error) {
	return &TestWorker{
		config:    cfg,
		sqsClient: sqsClient,
		queueURL:  cfg.SQSQueueURL,
		handler:   handler,
		logger:    logger,
		metrics:   metrics,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start starts the test worker
func (w *TestWorker) Start(ctx context.Context) error {
	go w.pollSQS(ctx)
	return nil
}

// Stop stops the test worker
func (w *TestWorker) Stop(ctx context.Context) error {
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})
	return nil
}

// pollSQS polls SQS (simplified version for testing)
func (w *TestWorker) pollSQS(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			w.receiveAndProcessMessages(ctx)
		}
	}
}

// receiveAndProcessMessages receives and processes messages (simplified)
func (w *TestWorker) receiveAndProcessMessages(ctx context.Context) {
	receiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(w.queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     0,
	}

	resp, err := w.sqsClient.ReceiveMessage(ctx, receiveInput)
	if err != nil {
		w.logger.ErrorLog(ctx, err, nil).Error("Failed to receive messages")
		return
	}

	if len(resp.Messages) == 0 {
		return
	}

	for _, msg := range resp.Messages {
		event, err := w.parseEvent(msg)
		if err != nil {
			w.logger.ErrorLog(ctx, err, nil).Error("Failed to parse event")
			continue
		}

		// Process event
		if err := w.handler.Handle(ctx, event); err != nil {
			w.logger.ErrorLog(ctx, err, nil).Error("Failed to handle event")
			continue
		}

		// Delete message
		w.deleteMessage(ctx, msg.ReceiptHandle)
	}
}

// parseEvent parses SQS message to event (simplified)
func (w *TestWorker) parseEvent(msg types.Message) (*eventTypes.Event, error) {
	if msg.Body == nil {
		return nil, fmt.Errorf("message body is nil")
	}

	var event eventTypes.Event
	if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	return &event, nil
}

// deleteMessage deletes message from SQS
func (w *TestWorker) deleteMessage(ctx context.Context, receiptHandle *string) {
	deleteInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(w.queueURL),
		ReceiptHandle: receiptHandle,
	}

	_, err := w.sqsClient.DeleteMessage(ctx, deleteInput)
	if err != nil {
		w.logger.ErrorLog(ctx, err, nil).Error("Failed to delete message")
	}
}
