# solana-exporter

This project monitors Solana RPC metrics using Docker and Docker Compose. It also integrates with Prometheus and Grafana for visualization.

## Metrics

This implementation includes:

1. Node Health Metrics:

- Health status
- Version information
- Uptime tracking
- Restart monitoring

2. Slot and Block Metrics:

- Slot heights and differences
- Block processing times
- Chain synchronization status

3. System Resource Metrics:

- CPU usage by core
- Memory usage stats
- Disk usage and IOPS
- Network IO
- File descriptors
