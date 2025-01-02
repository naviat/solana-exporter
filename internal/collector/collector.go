package collector

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "solana-rpc-monitor/config"
    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/internal/modules/health"
    "solana-rpc-monitor/internal/modules/performance"
    "solana-rpc-monitor/internal/modules/rpc"
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
    var modules []ModuleCollector

    // Only add health collector if enabled
    if c.config.RPC.HealthCheckEnabled {
        modules = append(modules, health.NewCollector(c.client, c.metrics, c.nodeLabels))
    }

    // Add other collectors
    modules = append(modules,
        rpc.NewCollector(c.client, c.metrics, c.nodeLabels, c.config),
        performance.NewCollector(c.client, c.metrics, c.nodeLabels),
    )

    c.modules = modules
}

func (c *Collector) detectNetwork(ctx context.Context) (string, error) {
    var genesisHash string
    if err := c.client.Call(ctx, "getGenesisHash", nil, &genesisHash); err != nil {
        return "", fmt.Errorf("failed to get genesis hash: %w", err)
    }

    // Known genesis hashes
    switch genesisHash {
    case "5eykt4UsFv8P8NJdTREpY1vzqKqZKvdpKuc147dw2N9d":
        return "mainnet-beta", nil
    case "EtWTRABZaYq6iMfeYKouRu166VU2xqa1wcaWoxPkrZBG":
        return "devnet", nil
    case "4uhcVJyU9pJkvQyS88uRDiswHXSCkY3zQawwpjk2NsNY":
        return "testnet", nil
    default:
        return "", fmt.Errorf("unknown network genesis hash: %s", genesisHash)
    }
}

func (c *Collector) Run(ctx context.Context) {
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    log.Printf("Starting collector with %d modules, interval: %v", len(c.modules), c.interval)

    for {
        select {
        case <-ctx.Done():
            log.Println("Collector received shutdown signal")
            return
        case <-ticker.C:
            if err := c.Collect(ctx); err != nil {
                log.Printf("Error during collection: %v", err)
            }
        }
    }
}

func (c *Collector) Collect(ctx context.Context) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(c.modules))
    semaphore := make(chan struct{}, c.config.Collector.ConcurrentModules)

    // Run each module collector in its own goroutine
    for _, module := range c.modules {
        wg.Add(1)
        go func(m ModuleCollector) {
            defer wg.Done()

            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            // Create timeout context for each collector
            moduleCtx, cancel := context.WithTimeout(ctx, c.config.Collector.TimeoutPerModule)
            defer cancel()

            // Track collection time
            start := time.Now()
            err := m.Collect(moduleCtx)
            duration := time.Since(start)

            // Record collection metrics
            c.recordCollectionMetrics(m.Name(), duration, err)

            if err != nil {
                select {
                case errCh <- fmt.Errorf("module %s: %w", m.Name(), err):
                default:
                    log.Printf("Error collecting %s metrics: %v", m.Name(), err)
                }
            }
        }(module)
    }

    // Wait for all collectors to finish
    wg.Wait()
    close(errCh)

    // Collect any errors
    var errs []error
    for err := range errCh {
        if err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("multiple collection errors: %v", errs)
    }

    return nil
}

func (c *Collector) recordCollectionMetrics(moduleName string, duration time.Duration, err error) {
    // We no longer track collection metrics in the new structure
    if err != nil {
        log.Printf("Error collecting RPC %s metrics: %v", moduleName, err)
    }
}

func (c *Collector) GetStatus() map[string]interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()

    return map[string]interface{}{
        "running":      true,
        "module_count": len(c.modules),
        "start_time":   c.startTime,
        "uptime":       time.Since(c.startTime).String(),
        "node_labels":  c.nodeLabels,
    }
}
