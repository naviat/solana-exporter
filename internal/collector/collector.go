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

type ModuleCollector interface {
    Collect(ctx context.Context) error
    Name() string
}

type Collector struct {
    client     *solana.Client
    metrics    *metrics.Metrics
    config     *config.Config
    modules    []ModuleCollector
    interval   time.Duration
    mu         sync.RWMutex
    startTime  time.Time
    nodeLabels map[string]string
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, cfg *config.Config, labels map[string]string) *Collector {
	if labels == nil {
        labels = make(map[string]string)
    }
    c := &Collector{
        client:     client,
        metrics:    metrics,
        config:     cfg,
        interval:   cfg.Collector.Interval,
        startTime:  time.Now(),
        nodeLabels: labels,
    }

    c.initializeModules()
    return c
}

func (c *Collector) initializeModules() {
    c.modules = []ModuleCollector{
        rpc.NewCollector(c.client, c.metrics, c.nodeLabels),
        health.NewCollector(c.client, c.metrics, c.nodeLabels),
        performance.NewCollector(c.client, c.metrics, c.nodeLabels),
        system.NewCollector(c.metrics, c.config.System, c.nodeLabels),
    }
}

func (c *Collector) Run(ctx context.Context) {
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    errorCh := make(chan error, len(c.modules))
    log.Printf("Starting collector with %d modules, interval: %v", len(c.modules), c.interval)

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
                c.recordError(err)
            }
        }
    }
}

func (c *Collector) collect(ctx context.Context, errorCh chan<- error) {
    var wg sync.WaitGroup
    
    moduleCtx, cancel := context.WithTimeout(ctx, c.config.Collector.TimeoutPerModule)
    defer cancel()

    semaphore := make(chan struct{}, c.config.Collector.ConcurrentModules)
    
    for _, module := range c.modules {
        wg.Add(1)
        go func(m ModuleCollector) {
            defer wg.Done()
            semaphore <- struct{}{} // Acquire
            defer func() { <-semaphore }() // Release

            start := time.Now()
            err := m.Collect(moduleCtx)
            duration := time.Since(start)

            c.recordModuleMetrics(m.Name(), duration, err)

            if err != nil {
                select {
                case errorCh <- err:
                default:
                    log.Printf("Error collecting %s metrics: %v", m.Name(), err)
                }
            }
        }(module)
    }

    wg.Wait()
}

func (c *Collector) recordModuleMetrics(moduleName string, duration time.Duration, err error) {
    labels := []string{
        c.nodeLabels["node_address"],
        moduleName,
    }

    c.metrics.CollectionDuration.WithLabelValues(labels...).Observe(duration.Seconds())

    if err == nil {
        c.metrics.CollectionSuccess.WithLabelValues(labels...).Inc()
    } else {
        c.metrics.CollectionErrors.WithLabelValues(labels...).Inc()
    }
}

func (c *Collector) recordError(err error) {
    c.metrics.CollectorErrors.Inc()
    // Additional error handling if needed
}

func (c *Collector) GetStatus() map[string]interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()

    return map[string]interface{}{
        "running":      true,
        "module_count": len(c.modules),
        "start_time":   c.startTime,
        "uptime":      time.Since(c.startTime).String(),
        "node_labels": c.nodeLabels,
    }
}
