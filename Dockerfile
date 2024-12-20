FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o solana-monitor ./cmd/monitor

FROM alpine:3.18

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/solana-monitor .
COPY config/config.yaml ./config/

# Run as non-root user
RUN adduser -D -u 1000 monitor
USER monitor

EXPOSE 9090

ENTRYPOINT ["./solana-monitor"]