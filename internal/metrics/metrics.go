package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // Slot/Block Metrics
    CurrentSlot    *prometheus.GaugeVec   // Current processed slot
    NetworkSlot    *prometheus.GaugeVec   // Network's highest slot (from reference node)
    SlotBehind     *prometheus.GaugeVec   // How many slots behind reference node
    BlockTime      *prometheus.GaugeVec   // Time since last block
    
    // Node Status Metrics
    NodeHealth     *prometheus.GaugeVec   // Health status (1=healthy, 0=unhealthy)
    NodeVersion    *prometheus.GaugeVec   // Node version info
    
    // RPC Performance Metrics
    RPCLatency     *prometheus.HistogramVec  // RPC response times
    RPCErrors      *prometheus.CounterVec    // RPC error counts by type
    RPCRequests    *prometheus.CounterVec    // Total RPC requests
    RPCInflightRequests    *prometheus.GaugeVec // Metric for inflight requests
    
    // WebSocket Metrics
    WSConnections          *prometheus.GaugeVec    // Active WS connections
    WSSubscriptions        *prometheus.GaugeVec    // Active subscriptions by type
    WSMessageRate          *prometheus.CounterVec  // Message rate by type
    WSErrors              *prometheus.CounterVec   // WebSocket errors by type
    WSLatency             *prometheus.HistogramVec // WebSocket message latency
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // Slot/Block Metrics
        CurrentSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_current_slot",
                Help: "Current processed slot number",
            },
            []string{"endpoint", "commitment"},
        ),
        NetworkSlot: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_network_slot",
                Help: "Network's highest known slot",
            },
            []string{"endpoint"},
        ),
        SlotBehind: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_behind",
                Help: "Number of slots behind the reference node",
            },
            []string{"endpoint"},
        ),
        BlockTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_time_seconds",
                Help: "Time since last block in seconds",
            },
            []string{"endpoint"},
        ),

        // Node Status Metrics
        NodeHealth: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_health",
                Help: "Node health status (1 = healthy, 0 = unhealthy)",
            },
            []string{"endpoint"},
        ),
        NodeVersion: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_node_version",
                Help: "Node version information",
            },
            []string{"endpoint", "version"},
        ),

        // RPC Performance Metrics
        RPCLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_latency_seconds",
                Help: "RPC method latency in seconds",
                // Adjusted buckets to better match Solana RPC timing patterns
                Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
            },
            []string{"endpoint", "method"},
        ),
        RPCErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_errors_total",
                Help: "Total RPC errors by type and method",
            },
            []string{"endpoint", "method", "error_type"},
        ),
        RPCRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_requests_total",
                Help: "Total RPC requests by method",
            },
            []string{"endpoint", "method"},
        ),
        RPCInflightRequests: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_inflight_requests",
                Help: "Number of in-flight RPC requests",
            },
            []string{"endpoint"},
        ),

        // WebSocket Metrics
        WSConnections: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_ws_connections",
                Help: "Number of active WebSocket connections",
            },
            []string{"endpoint"},
        ),
        WSSubscriptions: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_ws_subscriptions",
                Help: "Number of active WebSocket subscriptions",
            },
            []string{"endpoint", "subscription_type"},
        ),
        WSMessageRate: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_ws_messages_total",
                Help: "Total WebSocket messages by type",
            },
            []string{"endpoint", "direction"},
        ),
        WSErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_ws_errors_total",
                Help: "Total WebSocket errors by type",
            },
            []string{"endpoint", "error_type"},
        ),
        WSLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_ws_latency_seconds",
                Help: "WebSocket message latency in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
            },
            []string{"endpoint", "message_type"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotBehind,
        m.BlockTime,
        m.NodeHealth,
        m.NodeVersion,
        m.RPCLatency,
        m.RPCErrors,
        m.RPCRequests,
        m.WSConnections,
        m.WSSubscriptions,
        m.WSMessageRate,
        m.WSErrors,
        m.WSLatency,
    )

    return m
}
