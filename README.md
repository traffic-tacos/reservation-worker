# Reservation Worker

Asynchronous event processor for Traffic Tacos reservation system. Handles reservation expiry and payment result events via SQS with KEDA-based auto-scaling.

## Overview

This worker service processes background events for the reservation system:
- **Reservation Expired**: Releases held inventory and updates reservation status
- **Payment Approved**: Confirms reservation and commits inventory
- **Payment Failed**: Cancels reservation and releases held inventory

## Architecture

- **Event Source**: AWS SQS (with EventBridge)
- **Processing**: Concurrent worker pool (20 goroutines default)
- **Communication**:
  - gRPC with inventory-svc (proto-contracts reservationv1)
  - REST with reservation-api (with commonv1 types for consistency)
- **Auto-scaling**: KEDA based on SQS queue backlog
- **Observability**: OpenTelemetry, Prometheus, structured logging (zap)

## Features

- ✅ **Event Processing**: 3 event types (expired, approved, failed) with worker pool
- ✅ **AWS Integration**: SDK v2, profile auth (tacos), Secret Manager support
- ✅ **Proto-contracts**: Full gRPC integration with reservationv1 services
- ✅ **Resilience**: Exponential backoff retry (1s→2s→4s→8s, max 5 attempts)
- ✅ **Observability**: OpenTelemetry tracing, Prometheus metrics, structured logging
- ✅ **Development**: grpcui debugging (:8041), local environment setup
- ✅ **Production**: Docker containerization (linux/arm64), KEDA-compatible stateless design
- ✅ **Quality**: Unit/integration tests, idempotent operations, graceful shutdown

## Quick Start

### Prerequisites

```bash
# Install Go 1.23+
brew install go

# Install grpcui (optional, for debugging)
go install github.com/fullstorydev/grpcui/cmd/grpcui@latest

# AWS CLI configured with 'tacos' profile
aws configure --profile tacos
```

### Local Development

1. **Clone and setup**:
```bash
git clone https://github.com/traffic-tacos/reservation-worker.git
cd reservation-worker

# Install dependencies
make init

# Copy environment file
cp .env.example .env.local
# Edit .env.local with your AWS credentials and endpoints
```

2. **Run locally**:
```bash
# With environment file
make run-with-env

# Or with inline env vars
AWS_PROFILE=tacos make run

# Debug with grpcui (in another terminal)
make grpcui
```

3. **Build and test**:
```bash
# Run all checks (format, lint, test)
make verify

# Build binary
make build

# Run tests with coverage
make test-coverage

# Build Docker image
make docker-build
```

## Configuration

Environment variables (see `.env.example`):

```bash
# AWS Configuration
AWS_PROFILE=tacos
AWS_REGION=ap-northeast-2
USE_SECRET_MANAGER=false
SECRET_NAME=traffictacos/reservation-worker

# SQS Configuration
SQS_QUEUE_URL=https://sqs.ap-northeast-2.amazonaws.com/123456789/reservation-events
SQS_WAIT_TIME=20  # Long polling

# Worker Configuration
WORKER_CONCURRENCY=20  # Number of goroutines
MAX_RETRIES=5
BACKOFF_BASE_MS=1000

# External Services
INVENTORY_GRPC_ADDR=inventory-svc:8020
RESERVATION_API_BASE=http://reservation-api:8010

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
LOG_LEVEL=info  # debug, info, warn, error

# Server
SERVER_PORT=8040      # HTTP metrics and health
GRPC_DEBUG_PORT=8041  # gRPC debugging (grpcui)
```

## Event Schema

All events follow AWS EventBridge → SQS structure:
```json
{
  "id": "evt_123",
  "type": "reservation.expired | payment.approved | payment.failed",
  "source": "reservation-api | payment-sim-api",
  "detail": {
    "reservation_id": "rsv_456",
    "event_id": "evt_789",
    "qty": 2,
    "seat_ids": ["A1", "A2"]
  },
  "time": "2025-01-23T10:00:00Z",
  "trace_id": "trace_abc"
}
```

### Event Processing Workflows

1. **reservation.expired** → **inventory-svc** + **reservation-api**:
   ```
   1. gRPC: ReleaseHold(event_id, reservation_id, qty, seat_ids)
   2. REST: PATCH /internal/reservations/{id} {"status": "EXPIRED"}
   ```

2. **payment.approved** → **reservation-api** + **inventory-svc**:
   ```
   1. REST: PATCH /internal/reservations/{id} {"status": "CONFIRMED"}
   2. gRPC: CommitReservation(event_id, reservation_id, payment_intent_id)
   ```

3. **payment.failed** → **reservation-api** + **inventory-svc**:
   ```
   1. REST: PATCH /internal/reservations/{id} {"status": "CANCELLED"}
   2. gRPC: ReleaseHold(event_id, reservation_id, qty, seat_ids)
   ```

## Deployment

### Docker

```bash
# Build image
docker build -t reservation-worker:latest .

# Run with env file
docker run --env-file .env.local -p 8040:8040 -p 8041:8041 reservation-worker:latest

# Push to registry
docker tag reservation-worker:latest ghcr.io/traffic-tacos/reservation-worker:latest
docker push ghcr.io/traffic-tacos/reservation-worker:latest
```

### Kubernetes with KEDA

The service is designed to work with KEDA for auto-scaling based on SQS queue depth:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: reservation-worker-scaler
spec:
  scaleTargetRef:
    name: reservation-worker
  minReplicaCount: 0
  maxReplicaCount: 50
  triggers:
  - type: aws-sqs-queue
    metadata:
      queueURL: ${SQS_QUEUE_URL}
      queueLength: "10"
      awsRegion: "ap-northeast-2"
```

## Monitoring

### Health Checks

- `GET :8040/health` - Liveness probe
- `GET :8040/ready` - Readiness probe
- `GET :8040/metrics` - Prometheus metrics
- `grpcui :8041` - gRPC debugging interface

### Metrics

Key metrics exposed:
- `worker_events_total{type,outcome}` - Event processing counter
- `worker_latency_seconds{type}` - Processing duration histogram
- `sqs_poll_errors_total` - SQS polling errors
- `worker_active_count` - Active worker goroutines

### Logging

Structured JSON logging with fields:
- `ts`: Timestamp
- `level`: Log level
- `event_type`: Event type being processed
- `reservation_id`: Reservation identifier
- `trace_id`: Distributed trace ID
- `pod_name`: Kubernetes pod name

## Development

### Project Structure

```
reservation-worker/
├── cmd/reservation-worker/     # Main application entry
├── internal/
│   ├── client/                # External service clients
│   │   ├── inventory.go       # gRPC inventory client (proto-contracts)
│   │   └── reservation.go     # REST reservation client
│   ├── config/                # Configuration management
│   │   ├── config.go          # Environment variable loader
│   │   └── secrets.go         # AWS Secret Manager integration
│   ├── handler/               # Event handlers
│   │   ├── event.go           # Event structures and parsing
│   │   ├── expired.go         # Reservation expiry handler
│   │   ├── approved.go        # Payment approval handler
│   │   └── failed.go          # Payment failure handler
│   ├── observability/         # Observability stack
│   │   ├── logger.go          # Structured logging (zap)
│   │   ├── metrics.go         # Prometheus metrics
│   │   └── tracing.go         # OpenTelemetry tracing
│   ├── retry/                 # Exponential backoff retry logic
│   ├── server/                # gRPC debugging server (grpcui)
│   └── worker/                # Worker pool implementation
│       ├── poller.go          # SQS event polling
│       ├── dispatcher.go      # Event routing and retry
│       └── worker.go          # Individual worker goroutines
├── test/                      # Unit and integration tests
├── Dockerfile                 # Multi-stage container build
├── Makefile                   # Build automation and dev tools
├── .env.local/.env.example    # Environment configuration
└── README.md                  # Project documentation
```

### Testing

```bash
# Unit tests
make test

# Integration tests (requires LocalStack)
make localstack  # Start LocalStack
make test-integration

# Load testing
make test-load
```

### Debugging

1. **grpcui**: Interactive gRPC debugging
```bash
make grpcui
# Opens browser at http://localhost:8081 for gRPC interface at :8041
```

2. **Trace Context**: All events propagate trace IDs for distributed tracing

3. **Local SQS**: Use LocalStack for local development
```bash
make localstack
# SQS available at http://localhost:4566
```

## Troubleshooting

### Common Issues

1. **SQS Access Denied**:
   - Check AWS_PROFILE is set to 'tacos'
   - Verify SQS queue URL is correct
   - Ensure IAM permissions for SQS:ReceiveMessage, DeleteMessage

2. **gRPC Connection Failed**:
   - Check inventory-svc is running
   - Verify INVENTORY_GRPC_ADDR is correct
   - Check network connectivity

3. **High Memory Usage**:
   - Reduce WORKER_CONCURRENCY
   - Check for memory leaks with pprof

### Debug Mode

Enable debug logging:
```bash
LOG_LEVEL=debug make run
```

## Contributing

1. Fork the repository
2. Create feature branch
3. Run `make verify` before commit
4. Submit pull request

## License

Copyright © 2025 Traffic Tacos