package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/traffic-tacos/reservation-worker/internal/client"
	"github.com/traffic-tacos/reservation-worker/internal/config"
	"github.com/traffic-tacos/reservation-worker/internal/handler"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.uber.org/zap"
)

// Dispatcher manages worker goroutines and dispatches events to handlers
type Dispatcher struct {
	concurrency       int
	eventsChan        chan *handler.Event
	workerPool        chan chan *handler.Event
	workers           []*Worker
	wg                sync.WaitGroup
	stopChan          chan struct{}
	logger            *observability.Logger
	metrics           *observability.Metrics
	expiredHandler    *handler.ExpiredHandler
	approvedHandler   *handler.ApprovedHandler
	failedHandler     *handler.FailedHandler
	config            *config.Config
}

// NewDispatcher creates a new event dispatcher
func NewDispatcher(
	config *config.Config,
	inventoryClient *client.InventoryClient,
	reservationClient *client.ReservationClient,
	logger *observability.Logger,
	metrics *observability.Metrics,
) *Dispatcher {
	eventsChan := make(chan *handler.Event, config.WorkerConcurrency*2)
	workerPool := make(chan chan *handler.Event, config.WorkerConcurrency)

	// Create handlers
	expiredHandler := handler.NewExpiredHandler(inventoryClient, reservationClient, logger, metrics)
	approvedHandler := handler.NewApprovedHandler(inventoryClient, reservationClient, logger, metrics)
	failedHandler := handler.NewFailedHandler(inventoryClient, reservationClient, logger, metrics)

	return &Dispatcher{
		concurrency:     config.WorkerConcurrency,
		eventsChan:      eventsChan,
		workerPool:      workerPool,
		workers:         make([]*Worker, config.WorkerConcurrency),
		stopChan:        make(chan struct{}),
		logger:          logger,
		metrics:         metrics,
		expiredHandler:  expiredHandler,
		approvedHandler: approvedHandler,
		failedHandler:   failedHandler,
		config:          config,
	}
}

// Start starts the dispatcher and worker pool
func (d *Dispatcher) Start(ctx context.Context) error {
	d.logger.Info("Starting event dispatcher",
		zap.Int("concurrency", d.concurrency),
	)

	// Start workers
	for i := 0; i < d.concurrency; i++ {
		worker := NewWorker(i, d.workerPool, d.logger, d.metrics, d)
		d.workers[i] = worker
		d.wg.Add(1)
		go func(w *Worker) {
			defer d.wg.Done()
			w.Start(ctx)
		}(worker)
	}

	// Start dispatcher loop
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.dispatch(ctx)
	}()

	// Update metrics
	d.metrics.SetActiveWorkers(float64(d.concurrency))

	return nil
}

// Stop stops the dispatcher and all workers
func (d *Dispatcher) Stop() {
	d.logger.Info("Stopping event dispatcher")
	close(d.stopChan)
	d.wg.Wait()
	d.metrics.SetActiveWorkers(0)
}

// GetEventsChan returns the events channel for SQS poller
func (d *Dispatcher) GetEventsChan() chan *handler.Event {
	return d.eventsChan
}

// dispatch dispatches events from the channel to available workers
func (d *Dispatcher) dispatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Dispatcher stopped due to context cancellation")
			return
		case <-d.stopChan:
			d.logger.Info("Dispatcher stopped")
			return
		case event := <-d.eventsChan:
			// Get an available worker
			select {
			case workerChan := <-d.workerPool:
				// Send event to worker
				select {
				case workerChan <- event:
					// Event dispatched successfully
				case <-time.After(5 * time.Second):
					d.logger.Error("Timeout sending event to worker",
						zap.String("event_type", event.Type),
						zap.String("event_id", event.ID),
					)
				}
			case <-time.After(30 * time.Second):
				d.logger.Error("No workers available for event",
					zap.String("event_type", event.Type),
					zap.String("event_id", event.ID),
				)
			}
		}
	}
}

// HandleEvent routes events to appropriate handlers with retry logic
func (d *Dispatcher) HandleEvent(ctx context.Context, event *handler.Event, attempt int) error {
	start := time.Now()

	// Add retry attempt to context/logging
	logger := d.logger.WithEvent(event.Type, "", "")
	logger = logger.With(zap.Int("attempt", attempt))

	logger.Info("Processing event",
		zap.String("event_type", event.Type),
		zap.String("event_id", event.ID),
		zap.Int("attempt", attempt),
	)

	var err error

	// Route to appropriate handler
	switch event.Type {
	case handler.EventTypeReservationExpired, handler.EventTypeReservationHoldExpired:
		err = d.expiredHandler.Handle(ctx, event)

	case handler.EventTypePaymentApproved:
		err = d.approvedHandler.Handle(ctx, event)

	case handler.EventTypePaymentFailed:
		err = d.failedHandler.Handle(ctx, event)

	default:
		err = fmt.Errorf("unknown event type: %s", event.Type)
		d.metrics.RecordEventProcessed(event.Type, observability.OutcomeInvalidPayload)
		logger.Error("Unknown event type", zap.String("event_type", event.Type))
		return err
	}

	// Record metrics and handle retry logic
	duration := time.Since(start)
	if err != nil {
		if attempt >= d.config.MaxRetries {
			// Max retries exceeded
			d.metrics.RecordEventProcessed(event.Type, observability.OutcomeFailed)
			d.metrics.RecordEventLatency(event.Type, duration.Seconds())
			logger.Error("Event processing failed after max retries",
				zap.Error(err),
				zap.String("event_type", event.Type),
				zap.String("event_id", event.ID),
				zap.Int("max_retries", d.config.MaxRetries),
			)
			return err
		}

		// Retry with backoff
		d.metrics.RecordEventProcessed(event.Type, observability.OutcomeRetried)
		backoffDuration := d.config.GetBackoffDuration(attempt)

		logger.Warn("Event processing failed, retrying",
			zap.Error(err),
			zap.String("event_type", event.Type),
			zap.String("event_id", event.ID),
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoffDuration),
		)

		// Wait before retry
		time.Sleep(backoffDuration)

		// Retry
		return d.HandleEvent(ctx, event, attempt+1)
	}

	// Success
	d.metrics.RecordEventProcessed(event.Type, observability.OutcomeSuccess)
	d.metrics.RecordEventLatency(event.Type, duration.Seconds())

	logger.Info("Event processed successfully",
		zap.String("event_type", event.Type),
		zap.String("event_id", event.ID),
		zap.Duration("duration", duration),
	)

	return nil
}