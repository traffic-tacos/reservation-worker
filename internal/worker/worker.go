package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"

	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/handler"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	eventTypes "github.com/traffic-tacos/reservation-worker/pkg/types"
)

// Worker manages the SQS message processing
type Worker struct {
	config      *config.Config
	sqsClient   *sqs.Client
	queueURL    string
	handler     handler.EventHandler
	logger      *observability.Logger
	metrics     *observability.Metrics
	workerCount int
	maxRetries  int
	backoffBase time.Duration

	// Worker pool management
	workerWg sync.WaitGroup
	jobChan  chan *eventTypes.Event
	stopChan chan struct{}
	stopOnce sync.Once
}

// NewWorker creates a new worker instance
func NewWorker(cfg *config.Config, handler handler.EventHandler, logger *observability.Logger, metrics *observability.Metrics) (*Worker, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	sqsClient := sqs.NewFromConfig(awsCfg)

	return &Worker{
		config:      cfg,
		sqsClient:   sqsClient,
		queueURL:    cfg.SQSQueueURL,
		handler:     handler,
		logger:      logger,
		metrics:     metrics,
		workerCount: cfg.WorkerConcurrency,
		maxRetries:  cfg.MaxRetries,
		backoffBase: cfg.BackoffBaseDuration,
		jobChan:     make(chan *eventTypes.Event, cfg.WorkerConcurrency*2), // Buffer for burst handling
		stopChan:    make(chan struct{}),
	}, nil
}

// Start begins the worker pool and SQS polling
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("Starting reservation worker")

	// Start worker pool
	for i := 0; i < w.workerCount; i++ {
		w.workerWg.Add(1)
		go w.worker(ctx, i)
	}

	// Update metrics
	w.metrics.SetWorkerPoolActive(float64(w.workerCount))

	// Start SQS poller
	go w.pollSQS(ctx)

	w.logger.Logger.Info("Worker started successfully", zap.Int("worker_count", w.workerCount))
	return nil
}

// Stop gracefully stops the worker
func (w *Worker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping reservation worker")

	// Signal stop
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})

	// Close job channel
	close(w.jobChan)

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		w.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.logger.Info("All workers stopped gracefully")
	case <-ctx.Done():
		w.logger.Warn("Context cancelled while waiting for workers to stop")
	}

	w.metrics.SetWorkerPoolActive(0)
	return nil
}

// pollSQS continuously polls SQS for messages
func (w *Worker) pollSQS(ctx context.Context) {
	ticker := time.NewTicker(time.Second * time.Duration(w.config.SQSWaitTime))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("SQS polling stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("SQS polling stopped")
			return
		case <-ticker.C:
			if err := w.receiveAndProcessMessages(ctx); err != nil {
				w.metrics.RecordSQSError()
				w.logger.ErrorLog(ctx, err, nil).Error("Failed to receive and process messages")
			}
		}
	}
}

// receiveAndProcessMessages receives messages from SQS and sends them to workers
func (w *Worker) receiveAndProcessMessages(ctx context.Context) error {
	receiveInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(w.queueURL),
		MaxNumberOfMessages: 10, // Receive up to 10 messages at once
		WaitTimeSeconds:     int32(w.config.SQSWaitTime),
		VisibilityTimeout:   30, // 30 seconds visibility timeout
	}

	resp, err := w.sqsClient.ReceiveMessage(ctx, receiveInput)
	if err != nil {
		return fmt.Errorf("failed to receive messages: %w", err)
	}

	if len(resp.Messages) == 0 {
		return nil // No messages to process
	}

	w.logger.Logger.Debug("Received messages from SQS", zap.Int("message_count", len(resp.Messages)))

	for _, msg := range resp.Messages {
		event, err := w.parseEvent(msg)
		if err != nil {
			w.logger.ErrorLog(ctx, err, map[string]interface{}{
				"message_id": *msg.MessageId,
			}).Error("Failed to parse event from message")

			// Send to DLQ (in a real implementation, this would be handled by SQS DLQ configuration)
			if err := w.deleteMessage(ctx, msg.ReceiptHandle); err != nil {
				w.logger.ErrorLog(ctx, err, map[string]interface{}{
					"message_id": *msg.MessageId,
				}).Error("Failed to delete malformed message")
			}
			continue
		}

		// Send to worker pool
		select {
		case w.jobChan <- event:
			w.logger.Logger.Debug("Message sent to worker pool",
				zap.String("message_id", *msg.MessageId),
				zap.String("event_type", event.Type.String()),
				zap.String("reservation_id", event.ReservationID))
		case <-ctx.Done():
			return ctx.Err()
		default:
			w.logger.Logger.Warn("Worker pool is full, message will be retried", zap.String("message_id", *msg.MessageId))
			// Don't delete the message, let it become visible again
		}
	}

	return nil
}

// parseEvent parses an SQS message into an Event
func (w *Worker) parseEvent(msg types.Message) (*eventTypes.Event, error) {
	if msg.Body == nil {
		return nil, fmt.Errorf("message body is nil")
	}

	var event eventTypes.Event
	if err := json.Unmarshal([]byte(*msg.Body), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Validate required fields
	if event.ID == "" {
		return nil, fmt.Errorf("event id is required")
	}
	if event.ReservationID == "" {
		return nil, fmt.Errorf("reservation_id is required")
	}
	if event.Type == "" {
		return nil, fmt.Errorf("event type is required")
	}

	// Store receipt handle for later deletion
	event.TraceID = *msg.ReceiptHandle

	return &event, nil
}

// worker processes events from the job channel
func (w *Worker) worker(ctx context.Context, workerID int) {
	defer w.workerWg.Done()

	w.logger.Logger.Info("Worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-ctx.Done():
			w.logger.Logger.Info("Worker stopped due to context cancellation", zap.Int("worker_id", workerID))
			return
		case <-w.stopChan:
			w.logger.Logger.Info("Worker stopped", zap.Int("worker_id", workerID))
			return
		case event, ok := <-w.jobChan:
			if !ok {
				w.logger.Logger.Info("Job channel closed, worker stopping", zap.Int("worker_id", workerID))
				return
			}

			w.processEventWithRetry(ctx, event, workerID)
		}
	}
}

// processEventWithRetry processes an event with retry logic
func (w *Worker) processEventWithRetry(ctx context.Context, event *eventTypes.Event, workerID int) {
	startTime := time.Now()

	// Create backoff strategy
	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.InitialInterval = w.backoffBase
	backoffStrategy.MaxInterval = time.Minute * 2
	backoffStrategy.MaxElapsedTime = time.Minute * 5

	var lastErr error
	attempt := 0

	operation := func() error {
		attempt++
		eventCtx := context.WithValue(ctx, "attempt", attempt)

		w.logger.EventLog(eventCtx, event.Type.String(), event.ReservationID, map[string]interface{}{
			"worker_id": workerID,
			"attempt":   attempt,
		}).Info("Processing event")

		err := w.handler.Handle(eventCtx, event)
		if err != nil {
			lastErr = err
			w.logger.EventLog(eventCtx, event.Type.String(), event.ReservationID, map[string]interface{}{
				"worker_id": workerID,
				"attempt":   attempt,
				"error":     err.Error(),
			}).Warn("Event processing failed, will retry")
			return err
		}

		// Success - delete message from SQS
		if err := w.deleteMessage(ctx, &event.TraceID); err != nil {
			w.logger.ErrorLog(ctx, err, map[string]interface{}{
				"reservation_id": event.ReservationID,
				"worker_id":      workerID,
			}).Error("Failed to delete processed message")
			// Don't return error here as the business logic succeeded
		}

		latency := time.Since(startTime).Seconds()
		w.metrics.RecordEventLatency(event.Type.String(), latency)

		w.logger.EventLog(eventCtx, event.Type.String(), event.ReservationID, map[string]interface{}{
			"worker_id":  workerID,
			"attempt":    attempt,
			"latency_ms": latency * 1000,
		}).Info("Event processed successfully")

		return nil
	}

	err := backoff.Retry(operation, backoff.WithContext(backoffStrategy, ctx))

	if err != nil {
		// Max retries reached
		w.metrics.RecordEvent(event.Type.String(), observability.OutcomeFailed.String())

		w.logger.EventLog(ctx, event.Type.String(), event.ReservationID, map[string]interface{}{
			"worker_id": workerID,
			"attempts":  attempt,
			"error":     lastErr.Error(),
		}).Error("Event processing failed after max retries")

		// In a real implementation, you might want to send to DLQ or take other actions
		// For now, we'll just log the failure
	}
}

// deleteMessage deletes a message from SQS
func (w *Worker) deleteMessage(ctx context.Context, receiptHandle *string) error {
	deleteInput := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(w.queueURL),
		ReceiptHandle: receiptHandle,
	}

	_, err := w.sqsClient.DeleteMessage(ctx, deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}
