package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the Solana RPC exporter
type Metrics struct {
    // RPC Performance Metrics
    RPCLatency *prometheus.HistogramVec    // Method execution time
    RPCRequests *prometheus.CounterVec     // Total request count
    RPCErrors *prometheus.CounterVec       // Error count by type
    RPCInFlight *prometheus.GaugeVec       // Current in-flight requests
    RPCQueueDepth *prometheus.GaugeVec     // Request queue depth
    
    // Method-Specific Performance
    AccountLookupLatency *prometheus.HistogramVec  // Account queries timing
    ProgramLookupLatency *prometheus.HistogramVec  // Program account queries timing
    BlockFetchLatency *prometheus.HistogramVec     // Block retrieval timing
    TransactionLatency *prometheus.HistogramVec    // Transaction lookup timing
    
    // Rate Limiting Metrics
    RPCRateLimits *prometheus.GaugeVec          // Rate limit ceiling
    RPCRateCurrentUsage *prometheus.GaugeVec    // Current rate usage
    RPCRateRemaining *prometheus.GaugeVec       // Remaining rate limit
    
    // WebSocket Connection Metrics
    WSConnections *prometheus.GaugeVec      // Active connections
    WSSubscriptions *prometheus.GaugeVec    // Active subscriptions by type
    WSMessages *prometheus.CounterVec       // Message count by direction
    WSErrors *prometheus.CounterVec         // WebSocket errors
    WSLatency *prometheus.HistogramVec      // Message processing time
    
    // Cache Performance Metrics
    CacheHitRate *prometheus.GaugeVec       // Cache hit percentage
    CacheMissRate *prometheus.GaugeVec      // Cache miss percentage
    CacheSize *prometheus.GaugeVec          // Current cache size
    CacheEvictions *prometheus.CounterVec   // Cache eviction count
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        // Initialize RPC metrics with appropriate buckets
        RPCLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "solana_rpc_latency_seconds",
                Help: "RPC method latency in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"node", "method"},
        ),
        RPCRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_requests_total",
                Help: "Total number of RPC requests by method",
            },
            []string{"node", "method"},
        ),
        RPCErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_rpc_errors_total",
                Help: "RPC errors by type and method",
            },
            []string{"node", "method", "error_type"},
        ),
        RPCInFlight: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_in_flight_requests",
                Help: "Current number of in-flight RPC requests",
            },
            []string{"node"},
        ),
        RPCRateLimits: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_rpc_rate_limit",
                Help: "RPC rate limit by method",
            },
            []string{"node", "method"},
        ),
        
        // WebSocket metrics
        WSConnections: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_ws_connections",
                Help: "Number of active WebSocket connections",
            },
            []string{"node"},
        ),
        WSSubscriptions: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_ws_subscriptions",
                Help: "Number of active WebSocket subscriptions by type",
            },
            []string{"node", "subscription_type"},
        ),
        WSMessages: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "solana_ws_messages_total",
                Help: "Total WebSocket messages by direction",
            },
            []string{"node", "direction"},
        ),
        
        // Cache metrics
        CacheHitRate: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_cache_hit_rate",
                Help: "Cache hit rate percentage by type",
            },
            []string{"node", "cache_type"},
        ),
        CacheMissRate: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "solana_cache_miss_rate",
                Help: "Cache miss rate percentage by type",
            },
            []string{"node", "cache_type"},
        ),
    }

    // Register all metrics with Prometheus
    reg.MustRegister(
        m.RPCLatency,
        m.RPCRequests,
        m.RPCErrors,
        m.RPCInFlight,
        m.RPCRateLimits,
        m.WSConnections,
        m.WSSubscriptions,
        m.WSMessages,
        m.CacheHitRate,
        m.CacheMissRate,
    )

    return m
}
