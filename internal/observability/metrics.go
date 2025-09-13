package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the reservation worker
type Metrics struct {
	// Event processing metrics
	EventsTotal   *prometheus.CounterVec
	EventsLatency *prometheus.HistogramVec

	// SQS metrics
	SQSPollErrors prometheus.Counter

	// Worker pool metrics
	WorkerPoolActive prometheus.Gauge
}

// NewMetrics creates and registers all metrics
func NewMetrics() *Metrics {
	metrics := &Metrics{
		EventsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "worker_events_total",
				Help: "Total number of events processed by type and outcome",
			},
			[]string{"type", "outcome"},
		),

		EventsLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "worker_latency_seconds",
				Help:    "Event processing latency in seconds",
				Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
			[]string{"type"},
		),

		SQSPollErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "sqs_poll_errors_total",
				Help: "Total number of SQS poll errors",
			},
		),

		WorkerPoolActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "worker_pool_active_gauge",
				Help: "Number of active workers in the pool",
			},
		),
	}

	return metrics
}

// RecordEvent records an event processing result
func (m *Metrics) RecordEvent(eventType, outcome string) {
	m.EventsTotal.WithLabelValues(eventType, outcome).Inc()
}

// RecordEventLatency records the latency of an event processing
func (m *Metrics) RecordEventLatency(eventType string, latency float64) {
	m.EventsLatency.With(prometheus.Labels{"type": eventType}).Observe(latency)
}

// RecordSQSError records an SQS polling error
func (m *Metrics) RecordSQSError() {
	m.SQSPollErrors.Inc()
}

// SetWorkerPoolActive sets the number of active workers
func (m *Metrics) SetWorkerPoolActive(count float64) {
	m.WorkerPoolActive.Set(count)
}

// EventOutcome represents the outcome of event processing
type EventOutcome string

const (
	OutcomeSuccess EventOutcome = "success"
	OutcomeRetried EventOutcome = "retried"
	OutcomeFailed  EventOutcome = "failed"
	OutcomeDropped EventOutcome = "dropped"
)

// String returns the string representation of EventOutcome
func (o EventOutcome) String() string {
	return string(o)
}
