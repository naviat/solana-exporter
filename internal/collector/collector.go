package collector

import (
    "context"
    "log"
    "sync"
    "time"

    "solana-rpc-monitor/config"
    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/internal/modules/health"
    "solana-rpc-monitor/internal/modules/performance"
    "solana-rpc-monitor/internal/modules/rpc"
    "solana-rpc-monitor/internal/modules/system"
    "solana-rpc-monitor/pkg/solana"
)

// Collector defines interfaces for module collectors
type ModuleCollector interface {
    Collect(ctx context.Context) error
    Name() string
}

// Collector orchestrates all metric collection
type Collector struct {
    client    *solana.Client
    metrics   *metrics.Metrics
    config    *config.Config
    modules   []ModuleCollector
    interval  time.Duration
    mu        sync.RWMutex
    startTime time.Time
}

// NewCollector creates a new collector instance
func NewCollector(client *solana.Client, metrics *metrics.Metrics, cfg *config.Config) *Collector {
    c := &Collector{
        client:    client,
        metrics:   metrics,
        config:    cfg,
        interval:  cfg.Collector.Interval,
        startTime: time.Now(),
    }

    // Initialize module collectors
    c.modules = []ModuleCollector{
        rpc.NewCollector(client, metrics),
        health.NewCollector(client, metrics),
        performance.NewCollector(client, metrics),
        system.NewCollector(metrics, cfg.System),
    }

    return c
}

// Run starts the collector
func (c *Collector) Run(ctx context.Context) {
    // Create ticker for regular collection
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    // Create error channel for collecting errors from goroutines
    errorCh := make(chan error, len(c.modules))

    log.Printf("Starting collector with %d modules, interval: %v", len(c.modules), c.interval)

    // Start collection loop
    for {
        select {
        case <-ctx.Done():
            log.Println("Collector received shutdown signal")
            return
        case <-ticker.C:
            c.collect(ctx, errorCh)
        case err := <-errorCh:
            if err != nil {
                log.Printf("Collection error: %v", err)
            }
        }
    }
}

// collect runs all module collectors concurrently
func (c *Collector) collect(ctx context.Context, errorCh chan<- error) {
    var wg sync.WaitGroup

    // Update uptime metric
    c.metrics.UptimeSeconds.Add(float64(c.interval.Seconds()))

    // Run each module collector in its own goroutine
    for _, module := range c.modules {
        wg.Add(1)
        go func(m ModuleCollector) {
            defer wg.Done()

            // Create timeout context for each collector
            moduleCtx, cancel := context.WithTimeout(ctx, c.interval)
            defer cancel()

            // Track collection time
            start := time.Now()
            err := m.Collect(moduleCtx)
            duration := time.Since(start)

            // Record collection duration and status
            c.recordCollectionMetrics(m.Name(), duration, err)

            if err != nil {
                select {
                case errorCh <- err:
                default:
                    // Channel is full, log the error
                    log.Printf("Error collecting %s metrics: %v", m.Name(), err)
                }
            }
        }(module)
    }

    // Wait for all collectors to finish
    wg.Wait()
}

// recordCollectionMetrics records metrics about the collection process itself
func (c *Collector) recordCollectionMetrics(moduleName string, duration time.Duration, err error) {
    labels := []string{moduleName}
    
    // Record collection duration
    c.metrics.CollectionDuration.WithLabelValues(moduleName).Observe(duration.Seconds())
    
    // Record collection status (success = 1, failure = 0)
    if err == nil {
        c.metrics.CollectionSuccess.WithLabelValues(moduleName).Inc()
        c.metrics.LastCollectionSuccess.WithLabelValues(moduleName).Set(1)
    } else {
        c.metrics.CollectionErrors.WithLabelValues(moduleName).Inc()
        c.metrics.LastCollectionSuccess.WithLabelValues(moduleName).Set(0)
    }

    // Update last collection timestamp
    c.metrics.LastCollectionTimestamp.WithLabelValues(moduleName).Set(float64(time.Now().Unix()))
}

// Status returns current collector status
type Status struct {
    Running        bool      `json:"running"`
    ModuleCount    int       `json:"module_count"`
    StartTime      time.Time `json:"start_time"`
    Uptime         string    `json:"uptime"`
    LastCollection time.Time `json:"last_collection"`
}

// GetStatus returns the current status of the collector
func (c *Collector) GetStatus() Status {
    c.mu.RLock()
    defer c.mu.RUnlock()

    return Status{
        Running:     true,
        ModuleCount: len(c.modules),
        StartTime:   c.startTime,
        Uptime:      time.Since(c.startTime).String(),
    }
}

// AddModule adds a new collector module
func (c *Collector) AddModule(module ModuleCollector) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.modules = append(c.modules, module)
}

// RemoveModule removes a collector module by name
func (c *Collector) RemoveModule(name string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    for i, m := range c.modules {
        if m.Name() == name {
            c.modules = append(c.modules[:i], c.modules[i+1:]...)
            return
        }
    }
}

// Close performs any necessary cleanup
func (c *Collector) Close() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Any cleanup needed for modules
    c.modules = nil
    return nil
}
