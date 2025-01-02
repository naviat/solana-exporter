# solana-rpc-monitor

This project monitors Solana RPC metrics using Docker and Docker Compose. It also integrates with Prometheus and Grafana for visualization.

## Metrics

This implementation includes:

1. RPC Performance Metrics:

- Latency by method
- Request/error counts
- Payload sizes
- In-flight requests

2. Node Health Metrics:

- Health status
- Version information
- Uptime tracking
- Restart monitoring

3. Slot and Block Metrics:

- Slot heights and differences
- Block processing times
- Chain synchronization status

4. Transaction Metrics:

- Success/error rates
- Confirmation times
- Throughput monitoring
- Queue sizes

5. WebSocket Metrics:

- Connection counts
- Message rates
- Error tracking
