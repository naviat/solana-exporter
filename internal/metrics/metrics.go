package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // Slot/Block Metrics
    CurrentSlot        *prometheus.GaugeVec   // Current processed slot
    NetworkSlot        *prometheus.GaugeVec   // Reference node's slot
    SlotBehind         *prometheus.GaugeVec   // How many slots behind reference node
    BlockHeight        *prometheus.GaugeVec   // Current block height
    BlockTime          *prometheus.GaugeVec   // Time since last block
    SlotsPerSecond     *prometheus.GaugeVec   // Slots processed per second
    SlotProcessingTime *prometheus.GaugeVec   // Time to process each slot
    
    // Node Status Metrics
    NodeHealth     *prometheus.GaugeVec   // Health status (1=healthy, 0=unhealthy)
    NodeVersion    *prometheus.GaugeVec   // Node version info
    
    // Performance Metrics
    TPS            *prometheus.GaugeVec   // Transactions per second
    TPSMax         *prometheus.GaugeVec   // Max TPS in sample period
    TPSMin         *prometheus.GaugeVec   // Min TPS in sample period
    TxCount        *prometheus.CounterVec // Total transaction count
    
    // RPC Metrics
    RPCLatency     *prometheus.HistogramVec  // RPC response times
    RPCErrors      *prometheus.CounterVec    // RPC error counts by type
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
                Help: "Reference node's slot number",
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
        BlockHeight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_block_height",
                Help: "Current block height",
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
        SlotsPerSecond: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slots_per_second",
                Help: "Number of slots processed per second",
            },
            []string{"endpoint"},
        ),
        SlotProcessingTime: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_slot_processing_seconds",
                Help: "Time to process each slot in seconds",
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

        // Performance Metrics
        TPS: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_tps_current",
                Help: "Current transactions per second",
            },
            []string{"endpoint"},
        ),
        TPSMax: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_tps_max",
                Help: "Maximum transactions per second in sample period",
            },
            []string{"endpoint"},
        ),
        TPSMin: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_tps_min",
                Help: "Minimum transactions per second in sample period",
            },
            []string{"endpoint"},
        ),
        TxCount: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_transaction_count_total",
                Help: "Total number of transactions processed",
            },
            []string{"endpoint"},
        ),

        // RPC Metrics
        RPCLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_latency_seconds",
                Help: "RPC method latency in seconds",
                Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
            },
            []string{"endpoint", "method"},
        ),
        RPCErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_errors_total",
                Help: "RPC errors by type and method",
            },
            []string{"endpoint", "method", "error_code"},
        ),
    }

    // Register all metrics
    reg.MustRegister(
        m.CurrentSlot,
        m.NetworkSlot,
        m.SlotBehind,
        m.BlockHeight,
        m.BlockTime,
        m.SlotsPerSecond,
        m.SlotProcessingTime,
        m.NodeHealth,
        m.NodeVersion,
        m.TPS,
        m.TPSMax,
        m.TPSMin,
        m.TxCount,
        m.RPCLatency,
        m.RPCErrors,
    )

    return m
}
