package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/handler"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.uber.org/zap"
)

// SQSPoller polls SQS for events and sends them to workers
type SQSPoller struct {
	sqsClient   *sqs.Client
	queueURL    string
	waitTime    int32
	logger      *observability.Logger
	metrics     *observability.Metrics
	eventsChan  chan *handler.Event
	stopChan    chan struct{}
	config      *config.Config
}

// NewSQSPoller creates a new SQS poller
func NewSQSPoller(
	sqsClient *sqs.Client,
	config *config.Config,
	logger *observability.Logger,
	metrics *observability.Metrics,
	eventsChan chan *handler.Event,
) *SQSPoller {
	return &SQSPoller{
		sqsClient:  sqsClient,
		queueURL:   config.SQSQueueURL,
		waitTime:   int32(config.SQSWaitTime),
		logger:     logger,
		metrics:    metrics,
		eventsChan: eventsChan,
		stopChan:   make(chan struct{}),
		config:     config,
	}
}

// Start begins polling SQS for messages
func (p *SQSPoller) Start(ctx context.Context) error {
	p.logger.Info("Starting SQS poller",
		zap.String("queue_url", p.queueURL),
		zap.Int32("wait_time", p.waitTime),
	)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("SQS poller stopped due to context cancellation")
			return ctx.Err()
		case <-p.stopChan:
			p.logger.Info("SQS poller stopped")
			return nil
		default:
			if err := p.pollOnce(ctx); err != nil {
				p.logger.Error("Error polling SQS", zap.Error(err))
				p.metrics.RecordSQSPollError()

				// Backoff on error
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// Stop stops the SQS poller
func (p *SQSPoller) Stop() {
	close(p.stopChan)
}

// pollOnce performs a single SQS polling operation
func (p *SQSPoller) pollOnce(ctx context.Context) error {
	// Use ReceiveMessage with long polling
	result, err := p.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(p.queueURL),
		MaxNumberOfMessages: 10, // Process up to 10 messages at once
		WaitTimeSeconds:     p.waitTime,
		MessageAttributeNames: []string{"All"},
		AttributeNames:       []types.QueueAttributeName{types.QueueAttributeNameAll},
	})
	if err != nil {
		return fmt.Errorf("failed to receive messages from SQS: %w", err)
	}

	if len(result.Messages) == 0 {
		// No messages received, continue polling
		return nil
	}

	p.logger.Debug("Received messages from SQS",
		zap.Int("message_count", len(result.Messages)),
	)

	// Process each message
	for _, message := range result.Messages {
		if err := p.processMessage(ctx, &message); err != nil {
			p.logger.Error("Failed to process SQS message",
				zap.Error(err),
				zap.String("message_id", aws.ToString(message.MessageId)),
			)
			continue
		}

		// Delete message from queue after successful processing
		if err := p.deleteMessage(ctx, &message); err != nil {
			p.logger.Error("Failed to delete SQS message",
				zap.Error(err),
				zap.String("message_id", aws.ToString(message.MessageId)),
			)
		}
	}

	return nil
}

// processMessage processes a single SQS message
func (p *SQSPoller) processMessage(ctx context.Context, message *types.Message) error {
	if message.Body == nil {
		return fmt.Errorf("message body is nil")
	}

	// Parse the message body as an event
	var event handler.Event
	if err := json.Unmarshal([]byte(*message.Body), &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Add tracing information if available
	if message.MessageAttributes != nil {
		if traceID, ok := message.MessageAttributes["TraceId"]; ok && traceID.StringValue != nil {
			event.TraceID = *traceID.StringValue
		}
	}

	// Add message metadata
	if event.ID == "" && message.MessageId != nil {
		event.ID = *message.MessageId
	}

	p.logger.Debug("Processing event",
		zap.String("event_type", event.Type),
		zap.String("event_id", event.ID),
		zap.String("trace_id", event.TraceID),
	)

	// Send event to worker pool for processing
	select {
	case p.eventsChan <- &event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout sending event to worker pool")
	}
}

// deleteMessage deletes a message from SQS
func (p *SQSPoller) deleteMessage(ctx context.Context, message *types.Message) error {
	_, err := p.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	p.logger.Debug("Deleted message from SQS",
		zap.String("message_id", aws.ToString(message.MessageId)),
	)

	return nil
}

// getMessageApproximateReceiveCount gets the approximate receive count from message attributes
func getMessageApproximateReceiveCount(message *types.Message) int {
	if message.Attributes == nil {
		return 0
	}

	if countStr, ok := message.Attributes["ApproximateReceiveCount"]; ok {
		if count, err := strconv.Atoi(countStr); err == nil {
			return count
		}
	}

	return 0
}