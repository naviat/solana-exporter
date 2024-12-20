package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    // RPC Performance Metrics
    RPCLatency          *prometheus.HistogramVec
    RPCRequests         *prometheus.CounterVec
    RPCErrors           *prometheus.CounterVec
    RPCInFlight         prometheus.Gauge
    RPCRequestSize      prometheus.Histogram
    RPCResponseSize     prometheus.Histogram

    // Node Health Metrics
    NodeHealth          *prometheus.GaugeVec
    NodeVersion         *prometheus.GaugeVec
    LastRestartTime     prometheus.Gauge
    UptimeSeconds       prometheus.Counter

    // Slot and Block Metrics
    CurrentSlot         prometheus.Gauge
    NetworkSlot         prometheus.Gauge
    SlotDiff           prometheus.Gauge
    BlockHeight        prometheus.Gauge
    BlockProcessingTime prometheus.Histogram
    
    // Transaction Metrics
    TxCount            prometheus.Counter
    TxSuccessRate      prometheus.Gauge
    TxErrorRate        prometheus.Gauge
    TxPerSlot         *prometheus.HistogramVec
    TxConfirmationTime prometheus.Histogram
    TxQueueSize        prometheus.Gauge
    TxThroughput       prometheus.Gauge
    
    // System Resource Metrics
    CPUUsage           *prometheus.GaugeVec
    MemoryUsage        *prometheus.GaugeVec
    DiskUsage          *prometheus.GaugeVec
    DiskIOPS           *prometheus.GaugeVec
    NetworkIO          *prometheus.CounterVec
    FileDescriptors    prometheus.Gauge
    GoroutineCount     prometheus.Gauge
    
    // Collection Metrics
    CollectionDuration  *prometheus.HistogramVec
    CollectionSuccess   *prometheus.CounterVec
    CollectionErrors    *prometheus.CounterVec
    CollectorErrors     prometheus.Counter
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
        RPCInFlight: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "solana_rpc_in_flight_requests",
                Help: "Number of in-flight RPC requests",
            },
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
            []string{"node", "version", "features"},
        ),

        // Slot Metrics
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

        // System Resource Metrics
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
        DiskUsage: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_disk_usage_bytes",
                Help: "Disk usage in bytes",
            },
            []string{"node", "mount", "type"},
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
        CollectionSuccess: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_collector_success_total",
                Help: "Total number of successful collections",
            },
            []string{"node", "module"},
        ),
        CollectionErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_collector_errors_total",
                Help: "Total number of collection errors",
            },
            []string{"node", "module"},
        ),
    }

    // Register metrics
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
        m.CPUUsage,
        m.MemoryUsage,
        m.DiskUsage,
        m.CollectionDuration,
        m.CollectionSuccess,
        m.CollectionErrors,
    )

    return m
}

// ResetMetrics resets all vectored metrics for a given node
func (m *Metrics) ResetMetrics(node string) {
    labels := prometheus.Labels{"node": node}
    
    // Reset vec metrics
    if m.RPCLatency != nil {
        m.RPCLatency.DeletePartialMatch(labels)
    }
    if m.RPCRequests != nil {
        m.RPCRequests.DeletePartialMatch(labels)
    }
    if m.RPCErrors != nil {
        m.RPCErrors.DeletePartialMatch(labels)
    }
    if m.NodeHealth != nil {
        m.NodeHealth.DeletePartialMatch(labels)
    }
    if m.NodeVersion != nil {
        m.NodeVersion.DeletePartialMatch(labels)
    }
    if m.CPUUsage != nil {
        m.CPUUsage.DeletePartialMatch(labels)
    }
    if m.MemoryUsage != nil {
        m.MemoryUsage.DeletePartialMatch(labels)
    }
    if m.DiskUsage != nil {
        m.DiskUsage.DeletePartialMatch(labels)
    }
}

// RemoveNodeMetrics removes all metrics for a given node
func (m *Metrics) RemoveNodeMetrics(node string) {
    m.ResetMetrics(node)
    // Reset non-vec metrics to their zero values
    if m.RPCInFlight != nil {
        m.RPCInFlight.Set(0)
    }
    if m.CurrentSlot != nil {
        m.CurrentSlot.Set(0)
    }
    if m.NetworkSlot != nil {
        m.NetworkSlot.Set(0)
    }
    if m.SlotDiff != nil {
        m.SlotDiff.Set(0)
    }
}
