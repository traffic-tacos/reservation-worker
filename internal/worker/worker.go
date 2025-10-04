package worker

import (
	"context"

	"github.com/traffic-tacos/reservation-worker/internal/handler"
	"github.com/traffic-tacos/reservation-worker/internal/observability"
	"go.uber.org/zap"
)

// Worker represents a worker goroutine that processes events
type Worker struct {
	id         int
	workerPool chan chan *handler.Event
	eventChan  chan *handler.Event
	logger     *observability.Logger
	metrics    *observability.Metrics
	dispatcher *Dispatcher
}

// NewWorker creates a new worker
func NewWorker(
	id int,
	workerPool chan chan *handler.Event,
	logger *observability.Logger,
	metrics *observability.Metrics,
	dispatcher *Dispatcher,
) *Worker {
	return &Worker{
		id:         id,
		workerPool: workerPool,
		eventChan:  make(chan *handler.Event),
		logger:     logger,
		metrics:    metrics,
		dispatcher: dispatcher,
	}
}

// Start starts the worker loop
func (w *Worker) Start(ctx context.Context) {
	w.logger.Debug("Starting worker", zap.Int("worker_id", w.id))

	for {
		// Register worker in pool
		w.workerPool <- w.eventChan

		select {
		case <-ctx.Done():
			w.logger.Debug("Worker stopped due to context cancellation", zap.Int("worker_id", w.id))
			return

		case event := <-w.eventChan:
			if event == nil {
				continue
			}

			w.logger.Debug("Worker processing event",
				zap.Int("worker_id", w.id),
				zap.String("event_type", event.Type),
				zap.String("event_id", event.ID),
			)

			// Process event with retry logic
			if err := w.dispatcher.HandleEvent(ctx, event, 1); err != nil {
				w.logger.Error("Worker failed to process event",
					zap.Error(err),
					zap.Int("worker_id", w.id),
					zap.String("event_type", event.Type),
					zap.String("event_id", event.ID),
				)
			}
		}
	}
}