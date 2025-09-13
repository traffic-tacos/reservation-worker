package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	traceapi "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SetupTracing initializes OpenTelemetry tracing
func SetupTracing(ctx context.Context, endpoint string) (*trace.TracerProvider, error) {
	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, err
	}

	// Create trace provider
	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(time.Second*5),
			trace.WithMaxExportBatchSize(512),
		),
		trace.WithSampler(trace.AlwaysSample()),
	)

	otel.SetTracerProvider(traceProvider)

	// Set global propagator for trace context
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return traceProvider, nil
}

// SetupMetrics initializes OpenTelemetry metrics
func SetupMetrics(ctx context.Context, endpoint string) (*metric.MeterProvider, error) {
	// Create OTLP metric exporter
	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, err
	}

	// Create meter provider
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(10*time.Second),
		)),
	)

	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}

// SetupRuntimeMetrics starts runtime metrics collection
func SetupRuntimeMetrics(ctx context.Context) error {
	return runtime.Start(
		runtime.WithMinimumReadMemStatsInterval(time.Second * 10),
	)
}

// GetSpanFromContext extracts span from context
func GetSpanFromContext(ctx context.Context) interface{} {
	// Return the span from the context
	return traceapi.SpanFromContext(ctx)
}

// Tracer returns the global tracer
func Tracer(name string) interface{} {
	return otel.Tracer(name)
}

// Meter returns the global meter
func Meter(name string) interface{} {
	return otel.Meter(name)
}
