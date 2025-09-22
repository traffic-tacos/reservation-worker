package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the reservation worker
type Metrics struct {
	EventsTotal         *prometheus.CounterVec
	LatencyHistogram    *prometheus.HistogramVec
	SQSPollErrors       prometheus.Counter
	ActiveWorkers       prometheus.Gauge
	ProcessingDuration  *prometheus.HistogramVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		EventsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "worker_events_total",
				Help: "Total number of events processed by type and outcome",
			},
			[]string{"type", "outcome"},
		),

		LatencyHistogram: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "worker_latency_seconds",
				Help:    "Event processing latency in seconds",
				Buckets: prometheus.DefBuckets, // Default: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
			[]string{"type"},
		),

		SQSPollErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "sqs_poll_errors_total",
				Help: "Total number of SQS polling errors",
			},
		),

		ActiveWorkers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "worker_active_goroutines",
				Help: "Current number of active worker goroutines",
			},
		),

		ProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "worker_processing_duration_seconds",
				Help:    "Time spent processing events by handler type",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
			},
			[]string{"handler", "outcome"},
		),
	}
}

// RecordEventProcessed records a processed event with outcome
func (m *Metrics) RecordEventProcessed(eventType, outcome string) {
	m.EventsTotal.WithLabelValues(eventType, outcome).Inc()
}

// RecordEventLatency records event processing latency
func (m *Metrics) RecordEventLatency(eventType string, seconds float64) {
	m.LatencyHistogram.WithLabelValues(eventType).Observe(seconds)
}

// RecordSQSPollError increments SQS polling error counter
func (m *Metrics) RecordSQSPollError() {
	m.SQSPollErrors.Inc()
}

// SetActiveWorkers sets the current number of active workers
func (m *Metrics) SetActiveWorkers(count float64) {
	m.ActiveWorkers.Set(count)
}

// RecordProcessingDuration records handler processing duration
func (m *Metrics) RecordProcessingDuration(handler, outcome string, seconds float64) {
	m.ProcessingDuration.WithLabelValues(handler, outcome).Observe(seconds)
}

// Outcome constants for metrics
const (
	OutcomeSuccess         = "success"
	OutcomeRetried         = "retried"
	OutcomeFailed          = "failed"
	OutcomeDropped         = "dropped"
	OutcomeInvalidPayload  = "invalid_payload"
	OutcomeDownstreamError = "downstream_error"
)