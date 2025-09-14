# Build stage
FROM golang:1.22-alpine AS builder

# Install git and ca-certificates (for HTTPS requests)
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

# Create appuser
ENV USER=appuser
ENV UID=10001

# Create the user
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

WORKDIR /build

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags='-w -s -extldflags "-static"' -a \
    -installsuffix cgo -o reservation-worker \
    ./cmd/reservation-worker

# Final stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the binary
COPY --from=builder /build/reservation-worker /reservation-worker

# Use an unprivileged user
USER appuser:appuser

# Expose port (if needed for health checks)
EXPOSE 8080

# Run the binary
ENTRYPOINT ["/reservation-worker"]

