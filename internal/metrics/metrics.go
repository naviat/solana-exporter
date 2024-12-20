package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for Solana RPC node monitoring
type Metrics struct {
    // RPC Performance Metrics
    RPCLatency          *prometheus.HistogramVec  // Latency by method
    RPCRequests         *prometheus.CounterVec    // Request count by method
    RPCErrors           *prometheus.CounterVec    // Error count by method and error type
    RPCInFlight         prometheus.Gauge          // Current in-flight requests
    RPCRequestSize      prometheus.Histogram      // Request payload size
    RPCResponseSize     prometheus.Histogram      // Response payload size

    // Node Health Metrics
    NodeHealth          *prometheus.GaugeVec      // Node health status
    NodeVersion         *prometheus.GaugeVec      // Node version info
    LastRestartTime     prometheus.Gauge          // Last node restart timestamp
    UptimeSeconds       prometheus.Counter        // Node uptime in seconds

    // Slot and Block Metrics
    CurrentSlot         prometheus.Gauge          // Current processed slot
    NetworkSlot         prometheus.Gauge          // Latest network slot
    SlotDiff           prometheus.Gauge          // Difference between network and node slot
    BlockHeight        prometheus.Gauge          // Current block height
    BlockProcessingTime prometheus.Histogram      // Time to process blocks
    
    // Transaction Metrics
    TxCount            prometheus.Counter         // Total transactions processed
    TxSuccessRate      prometheus.Gauge          // Transaction success rate
    TxErrorRate        prometheus.Gauge          // Transaction error rate
    TxPerSlot         *prometheus.HistogramVec   // Transactions per slot
    TxConfirmationTime prometheus.Histogram      // Transaction confirmation time
    TxQueueSize        prometheus.Gauge          // Size of transaction queue
    TxThroughput       prometheus.Gauge          // Transactions per second
    
    // System Resource Metrics
    CPUUsage           *prometheus.GaugeVec      // CPU usage by core
    MemoryUsage        *prometheus.GaugeVec      // Memory usage stats
    DiskUsage          *prometheus.GaugeVec      // Disk usage stats
    DiskIOPS           *prometheus.GaugeVec      // Disk IOPS
    NetworkIO          *prometheus.CounterVec    // Network IO stats
    FileDescriptors    prometheus.Gauge          // Open file descriptors
    GoroutineCount     prometheus.Gauge          // Number of goroutines
    
    // Cache Metrics
    CacheSize          *prometheus.GaugeVec      // Cache size by type
    CacheHitRate       *prometheus.GaugeVec      // Cache hit rate
    CacheMissRate      *prometheus.GaugeVec      // Cache miss rate
    CacheEvictions     *prometheus.CounterVec    // Cache evictions

    // Websocket Metrics
    WSConnections      prometheus.Gauge          // Active WebSocket connections
    WSMessageRate      *prometheus.CounterVec    // WebSocket message rate
    WSErrors           *prometheus.CounterVec    // WebSocket errors
}

// NewMetrics creates and registers all metrics
func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // RPC Performance Metrics
        RPCLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_latency_seconds",
                Help: "RPC request latency in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"method"},
        ),
        RPCRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_requests_total",
                Help: "Total number of RPC requests",
            },
            []string{"method"},
        ),
        RPCErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_errors_total",
                Help: "Total number of RPC errors",
            },
            []string{"method", "error_type"},
        ),
        RPCInFlight: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_rpc_in_flight_requests",
                Help: "Number of in-flight RPC requests",
            },
        ),
        RPCRequestSize: prometheus.NewHistogram(
            prometheus.HistogramOpts{
                Name: "solana_rpc_request_size_bytes",
                Help: "Size of RPC request payloads in bytes",
                Buckets: []float64{64, 256, 1024, 4096, 16384, 65536, 262144},
            },
        ),
        RPCResponseSize: prometheus.NewHistogram(
            prometheus.HistogramOpts{
                Name: "solana_rpc_response_size_bytes",
                Help: "Size of RPC response payloads in bytes",
                Buckets: []float64{64, 256, 1024, 4096, 16384, 65536, 262144},
            },
        ),

        // Node Health Metrics
        NodeHealth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_health",
                Help: "Node health status (1 = healthy, 0 = unhealthy)",
            },
            []string{"status"},
        ),
        NodeVersion: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_version",
                Help: "Node version information",
            },
            []string{"version", "features"},
        ),
        LastRestartTime: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_node_last_restart_timestamp",
                Help: "Timestamp of last node restart",
            },
        ),
        UptimeSeconds: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "solana_node_uptime_seconds",
                Help: "Node uptime in seconds",
            },
        ),

        // Slot and Block Metrics
        CurrentSlot: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_current_slot",
                Help: "Current processed slot",
            },
        ),
        NetworkSlot: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_network_slot",
                Help: "Latest network slot",
            },
        ),
        SlotDiff: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_slot_diff",
                Help: "Difference between network and node slot",
            },
        ),
        BlockProcessingTime: prometheus.NewHistogram(
            prometheus.HistogramOpts{
                Name: "solana_block_processing_seconds",
                Help: "Time to process blocks in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
            },
        ),

        // Transaction Metrics
        TxCount: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "solana_transactions_total",
                Help: "Total number of transactions processed",
            },
        ),
        TxSuccessRate: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_transaction_success_rate",
                Help: "Transaction success rate",
            },
        ),
        TxPerSlot: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_transactions_per_slot",
                Help: "Number of transactions per slot",
                Buckets: []float64{10, 50, 100, 200, 500, 1000, 2000, 5000},
            },
            []string{"status"},
        ),
        TxConfirmationTime: prometheus.NewHistogram(
            prometheus.HistogramOpts{
                Name: "solana_transaction_confirmation_seconds",
                Help: "Transaction confirmation time in seconds",
                Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
            },
        ),

        // System Resource Metrics
        CPUUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_cpu_usage_percent",
                Help: "CPU usage percentage",
            },
            []string{"core"},
        ),
        MemoryUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_memory_usage_bytes",
                Help: "Memory usage in bytes",
            },
            []string{"type"}, // heap, stack, etc.
        ),
        DiskUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_disk_usage_bytes",
                Help: "Disk usage in bytes",
            },
            []string{"mount", "type"},
        ),
        NetworkIO: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_network_io_bytes",
                Help: "Network IO in bytes",
            },
            []string{"direction"}, // in, out
        ),

        // WebSocket Metrics
        WSConnections: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_websocket_connections",
                Help: "Number of active WebSocket connections",
            },
        ),
        WSMessageRate: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_websocket_messages_total",
                Help: "Total WebSocket messages",
            },
            []string{"type"}, // subscribe, unsubscribe, notification
        ),
    }

    // Register all metrics with Prometheus
    reg.MustRegister(
        m.RPCLatency,
        m.RPCRequests,
        m.RPCErrors,
        m.RPCInFlight,
        m.RPCRequestSize,
        m.RPCResponseSize,
        m.NodeHealth,
        m.NodeVersion,
        m.LastRestartTime,
        m.UptimeSeconds,
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotDiff,
        m.BlockProcessingTime,
        m.TxCount,
        m.TxSuccessRate,
        m.TxPerSlot,
        m.TxConfirmationTime,
        m.CPUUsage,
        m.MemoryUsage,
        m.DiskUsage,
        m.NetworkIO,
        m.WSConnections,
        m.WSMessageRate,
    )

    return m
}

// ResetMetrics resets all metrics for a given node
func (m *Metrics) ResetMetrics(node string) {
    labels := prometheus.Labels{"node": node}
    m.RPCLatency.DeletePartialMatch(labels)
    m.RPCRequests.DeletePartialMatch(labels)
    m.RPCErrors.DeletePartialMatch(labels)
    m.NodeHealth.DeletePartialMatch(labels)
    m.CurrentSlot.DeletePartialMatch(labels)
    m.SlotDiff.DeletePartialMatch(labels)
    m.RPCInFlight.DeletePartialMatch(labels)
    m.RPCRequestSize.DeletePartialMatch(labels)
    m.RPCResponseSize.DeletePartialMatch(labels)
    m.NodeVersion.DeletePartialMatch(labels)
    m.LastRestartTime.DeletePartialMatch(labels)
    m.UptimeSeconds.DeletePartialMatch(labels)
    m.NetworkSlot.DeletePartialMatch(labels)
    m.BlockProcessingTime.DeletePartialMatch(labels)
    m.TxCount.DeletePartialMatch(labels)
    m.TxSuccessRate.DeletePartialMatch(labels)
    m.TxPerSlot.DeletePartialMatch(labels)
    m.TxConfirmationTime.DeletePartialMatch(labels)
    m.CPUUsage.DeletePartialMatch(labels)
    m.MemoryUsage.DeletePartialMatch(labels)
    m.DiskUsage.DeletePartialMatch(labels)
    m.NetworkIO.DeletePartialMatch(labels)
    m.WSConnections.DeletePartialMatch(labels)
    m.WSMessageRate.DeletePartialMatch(labels)
}

// RemoveNodeMetrics removes all metrics for a given node
func (m *Metrics) RemoveNodeMetrics(node string) {
    m.ResetMetrics(node)
}