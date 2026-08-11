# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy all source code
COPY . .

# Download dependencies and build
RUN go build -buildvcs=false -o server .

# Runtime stage
FROM alpine:3.23

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Copy binary from builder
COPY --from=builder /app/server .

# Create logs directory
RUN mkdir -p /app/logs && chown -R appuser:appuser /app

USER appuser

EXPOSE 3002

# NOT `-release=true`, and that is a deliberate change (TODO3 M3).
#
# This image is built by exactly one thing — compose.dev.yaml, the LOCAL dev
# stack. Production runs no containers at all: systemd runs the bare binary,
# and `deploy/thegathering.service` is where `-release=true` actually lives.
# This CMD is a leftover from the Docker production stack deleted in 2026-08,
# and its only surviving effect was to make the dev stack claim to be
# production — which silently overrode `GIN_MODE: debug` in compose.dev.yaml
# (the flag calls gin.SetMode and wins over the environment) and made
# `gin.Mode()` useless as a "this is not production" signal locally.
CMD ["./server"]
