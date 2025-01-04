FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /solana-exporter ./cmd/exporter

FROM alpine:3.19

COPY --from=builder /solana-exporter /usr/local/bin/
COPY config.yaml /etc/solana-exporter/

EXPOSE 9104

ENTRYPOINT ["/usr/local/bin/solana-exporter", "--config=/etc/solana-exporter/config.yaml"]
