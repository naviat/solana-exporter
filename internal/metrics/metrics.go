package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // RPC Performance Metrics
    RPCLatency          *prometheus.HistogramVec
    RPCRequests         *prometheus.CounterVec
    RPCErrors           *prometheus.CounterVec
    RPCInFlight         *prometheus.GaugeVec
    RPCRequestSize      *prometheus.HistogramVec
    RPCResponseSize     *prometheus.HistogramVec

    // Node Health Metrics
    NodeHealth          *prometheus.GaugeVec
    NodeVersion         *prometheus.GaugeVec
    LastRestartTime     *prometheus.GaugeVec
    UptimeSeconds       *prometheus.CounterVec

    // Slot and Block Metrics
    CurrentSlot         *prometheus.GaugeVec
    NetworkSlot         *prometheus.GaugeVec
    SlotDiff           *prometheus.GaugeVec
    BlockHeight        *prometheus.GaugeVec
    BlockProcessingTime *prometheus.HistogramVec
    
    // Transaction Metrics
    TxThroughput       *prometheus.GaugeVec
    TxSuccessRate      *prometheus.GaugeVec
    TxErrorRate        *prometheus.GaugeVec
    TxPerSlot          *prometheus.HistogramVec
    TxConfirmationTime *prometheus.HistogramVec
    TxQueueSize        *prometheus.GaugeVec
    
    // System Resource Metrics
    CPUUsage           *prometheus.GaugeVec
    MemoryUsage        *prometheus.GaugeVec
    DiskUsage          *prometheus.GaugeVec
    DiskIOPS           *prometheus.GaugeVec
    NetworkIO          *prometheus.CounterVec
    SystemFileDesc    *prometheus.GaugeVec
    GoroutineCount     *prometheus.GaugeVec

    // Collection Metrics
    CollectionDuration  *prometheus.HistogramVec
    CollectionSuccess   *prometheus.CounterVec
    CollectionErrors    *prometheus.CounterVec

    // Additional RPC Metrics
    RPCQueueDepth       *prometheus.GaugeVec     // Queue depth for RPC requests
    RPCRateLimit       *prometheus.CounterVec    // Rate limit hits
    RPCBatchSize       *prometheus.HistogramVec  // Size of batch requests
    RPCWebsocketConns  *prometheus.GaugeVec     // Active websocket connections

    // Additional Performance Metrics
    BlockTime          *prometheus.GaugeVec     // Time between blocks
    SlotProcessingTime *prometheus.HistogramVec // Time to process each slot
    TransactionRetries *prometheus.CounterVec   // Number of transaction retries
    BlockPropagation  *prometheus.HistogramVec // Block propagation time
    ValidatorCount    *prometheus.GaugeVec     // Number of active validators
    AccountsCache     *prometheus.GaugeVec     // Accounts cache statistics
    TransactionCache  *prometheus.GaugeVec     // Transaction cache statistics

    // Additional System Metrics
    IOWaitTime        *prometheus.GaugeVec     // IO wait time
    SwapUsage         *prometheus.GaugeVec     // Swap memory usage
    TCPConnections    *prometheus.GaugeVec     // TCP connection stats
    FileDescriptors   *prometheus.GaugeVec     // File descriptor usage
    ProcessStats      *prometheus.GaugeVec     // Process statistics
    SystemLoad        *prometheus.GaugeVec     // System load averages
    NetworkErrors     *prometheus.CounterVec   // Network error counts
    DiskLatency      *prometheus.HistogramVec  // Disk operation latency
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // RPC Performance Metrics
        RPCLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_latency_seconds",
                Help: "RPC request latency in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"node", "method"},
        ),
        RPCRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_requests_total",
                Help: "Total number of RPC requests",
            },
            []string{"node", "method"},
        ),
        RPCErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_errors_total",
                Help: "Total number of RPC errors",
            },
            []string{"node", "method", "error_type"},
        ),
        RPCInFlight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_in_flight_requests",
                Help: "Number of in-flight RPC requests",
            },
            []string{"node"},
        ),
        RPCRequestSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_request_size_bytes",
                Help: "Size of RPC request payloads in bytes",
                Buckets: []float64{64, 256, 1024, 4096, 16384, 65536, 262144},
            },
            []string{"node", "method"},
        ),

        // Node Health Metrics
        NodeHealth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_health",
                Help: "Node health status (1 = healthy, 0 = unhealthy)",
            },
            []string{"node", "status"},
        ),
        NodeVersion: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_version",
                Help: "Node version information",
            },
            []string{"node", "version", "feature_set"},
        ),

        // Slot Metrics
        CurrentSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_current_slot",
                Help: "Current slot height",
            },
            []string{"node"},
        ),
        NetworkSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_network_slot",
                Help: "Network slot height",
            },
            []string{"node"},
        ),
        SlotDiff: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_diff",
                Help: "Difference between network and node slot",
            },
            []string{"node"},
        ),

        // Transaction Metrics
        TxThroughput: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_transaction_throughput",
                Help: "Transaction throughput (TPS)",
            },
            []string{"node"},
        ),
        TxSuccessRate: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_transaction_success_rate",
                Help: "Transaction success rate percentage",
            },
            []string{"node"},
        ),

        // System Metrics
        CPUUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_cpu_usage_percent",
                Help: "CPU usage percentage",
            },
            []string{"node", "core"},
        ),
        MemoryUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_memory_usage_bytes",
                Help: "Memory usage in bytes",
            },
            []string{"node", "type"},
        ),

        // Collection Metrics
        CollectionDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_collector_duration_seconds",
                Help: "Time spent collecting metrics",
                Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10},
            },
            []string{"node", "module"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        m.RPCLatency,
        m.RPCRequests,
        m.RPCErrors,
        m.RPCInFlight,
        m.NodeHealth,
        m.NodeVersion,
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotDiff,
        m.TxThroughput,
        m.TxSuccessRate,
        m.CPUUsage,
        m.MemoryUsage,
        m.CollectionDuration,
    )

    return m
}
