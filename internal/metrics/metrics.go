package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // RPC Performance Metrics
    RPCLatency          *prometheus.HistogramVec    // Labels: ["node", "method"]
    RPCRequests         *prometheus.CounterVec      // Labels: ["node", "method"]
    RPCErrors           *prometheus.CounterVec      // Labels: ["node", "method", "error_type"]
    RPCInFlight         *prometheus.GaugeVec        // Labels: ["node"]
    RPCRequestSize      *prometheus.HistogramVec    // Labels: ["node", "method"]
    RPCResponseSize     *prometheus.HistogramVec    // Labels: ["node", "method"]
    RPCQueueDepth       *prometheus.GaugeVec        // Labels: ["node"]
    RPCRateLimit        *prometheus.CounterVec      // Labels: ["node"]
    RPCWebsocketConns   *prometheus.GaugeVec        // Labels: ["node"]

    // Node Health Metrics
    NodeHealth          *prometheus.GaugeVec        // Labels: ["node", "status"]
    NodeVersion         *prometheus.GaugeVec        // Labels: ["node", "version"]

    // Slot and Block Metrics
    CurrentSlot         *prometheus.GaugeVec        // Labels: ["node"]
    NetworkSlot         *prometheus.GaugeVec        // Labels: ["node"]
    SlotDiff            *prometheus.GaugeVec        // Labels: ["node"]
    BlockHeight         *prometheus.GaugeVec        // Labels: ["node"]
    BlockTime           *prometheus.GaugeVec        // Labels: ["node"]
    BlockProcessingTime *prometheus.HistogramVec    // Labels: ["node"]
    
    // Transaction Metrics
    TxThroughput        *prometheus.GaugeVec        // Labels: ["node"]
    TxSuccessRate       *prometheus.GaugeVec        // Labels: ["node"]
    TxErrorRate         *prometheus.GaugeVec        // Labels: ["node"]
    TxPerSlot           *prometheus.HistogramVec    // Labels: ["node"]
    TxConfirmationTime  *prometheus.HistogramVec    // Labels: ["node"]
    TransactionRetries  *prometheus.CounterVec      // Labels: ["node", "type"]
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
        RPCQueueDepth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_queue_depth",
                Help: "Queue depth for RPC requests",
            },
            []string{"node"},
        ),
        RPCRateLimit: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_rate_limit_hits_total",
                Help: "Number of times RPC rate limit was hit",
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
        RPCResponseSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_response_size_bytes",
                Help: "Size of RPC response payloads in bytes",
                Buckets: []float64{64, 256, 1024, 4096, 16384, 65536, 262144},
            },
            []string{"node", "method"},
        ),
        RPCWebsocketConns: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_websocket_connections",
                Help: "Number of active websocket connections",
            },
            []string{"node"},
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
            []string{"node", "version"},
        ),

        // Slot and Block Metrics
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
        BlockHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_height",
                Help: "Current block height",
            },
            []string{"node"},
        ),
        BlockTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_time_seconds",
                Help: "Time between blocks",
            },
            []string{"node"},
        ),
        BlockProcessingTime: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_block_processing_time_seconds",
                Help: "Time taken to process blocks",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
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
        TxErrorRate: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_transaction_error_rate",
                Help: "Transaction error rate percentage",
            },
            []string{"node"},
        ),
        TxPerSlot: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_transactions_per_slot",
                Help: "Number of transactions per slot",
                Buckets: []float64{10, 50, 100, 200, 500, 1000, 2000, 5000},
            },
            []string{"node"},
        ),
        TxConfirmationTime: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_transaction_confirmation_time_seconds",
                Help: "Time taken for transaction confirmation",
                Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
            },
            []string{"node"},
        ),
        TransactionRetries: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_transaction_retries_total",
                Help: "Number of transaction retries",
            },
            []string{"node", "type"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        m.RPCLatency,
        m.RPCRequests,
        m.RPCErrors,
        m.RPCInFlight,
        m.RPCRequestSize,
        m.RPCResponseSize,
        m.RPCQueueDepth,
        m.RPCRateLimit,
        m.RPCWebsocketConns,
        m.NodeHealth,
        m.NodeVersion,
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotDiff,
        m.BlockHeight,
        m.BlockTime,
        m.BlockProcessingTime,
        m.TxThroughput,
        m.TxSuccessRate,
        m.TxErrorRate,
        m.TxPerSlot,
        m.TxConfirmationTime,
        m.TransactionRetries,
    )

    return m
}
