package collector

import (
    "context"
    "fmt"
    "log"
    "sort"
    "sync"
    "time"

    "solana-exporter/config"
    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
    "solana-exporter/internal/modules/rpc"
    "solana-exporter/internal/modules/websocket"
    "solana-exporter/internal/modules/cache"
)

type ModuleCollector interface {
    Collect(ctx context.Context) error
    Name() string
    Priority() int
}

type Collector struct {
    client         *solana.Client
    metrics        *metrics.Metrics
    config         *config.Config
    modules        []ModuleCollector    // This field was missing
    interval       time.Duration
    mu             sync.RWMutex
    startTime      time.Time
    nodeLabels     map[string]string
    lastCollection time.Time
    healthStatus   bool
    batchSize      int
    batchInterval  time.Duration
    stats          *CollectorStats
}

type CollectorStats struct {
    TotalCollections    int64
    SuccessfulBatches   int64
    FailedBatches      int64
    LastError          error
    LastSuccessTime    time.Time
}

func NewCollector(client *solana.Client, metrics *metrics.Metrics, cfg *config.Config, labels map[string]string) *Collector {
    if labels == nil {
        labels = make(map[string]string)
    }

    c := &Collector{
        client:        client,
        metrics:       metrics,
        config:       cfg,
        interval:     cfg.Collector.Interval,
        batchSize:    cfg.Collector.BatchSize,
        batchInterval: cfg.Collector.BatchInterval,
        startTime:    time.Now(),
        nodeLabels:   labels,
        stats:        &CollectorStats{},
        modules:      make([]ModuleCollector, 0),
    }
    // Initialize modules right after creating the collector
    c.initializeModules()
    return c
}

// Initialize the collection modules with proper priorities
func (c *Collector) initializeModules() {
    // Create modules with priority-based ordering
    c.modules = []ModuleCollector{
        rpc.NewCollector(c.client, c.metrics, c.nodeLabels, c.config),        // Highest priority
        websocket.NewCollector(c.client, c.metrics, c.nodeLabels, c.config),  // Medium priority
        cache.NewCollector(c.client, c.metrics, c.nodeLabels, c.config),      // Lower priority
    }

    // Log initialization for debugging
    log.Printf("Initialized %d collection modules", len(c.modules))
    for _, module := range c.modules {
        log.Printf("Module loaded: %s (priority: %d)", module.Name(), module.Priority())
    }
}

func (c *Collector) Start(ctx context.Context) error {
    c.initializeModules()
    
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    log.Printf("Starting collector with batch size %d and interval %v", 
        c.batchSize, c.batchInterval)

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := c.Collect(ctx); err != nil {
                log.Printf("Collection error: %v", err)
                c.updateStats(false, err)
            } else {
                c.updateStats(true, nil)
            }
        }
    }
}

// Stop gracefully shuts down the collector
func (c *Collector) Stop() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Perform cleanup tasks
    for _, module := range c.modules {
        if stopper, ok := module.(interface{ Stop() error }); ok {
            if err := stopper.Stop(); err != nil {
                log.Printf("Error stopping module %s: %v", module.Name(), err)
            }
        }
    }

    return nil
}

func (c *Collector) Collect(ctx context.Context) error {
    collectionStart := time.Now()
    defer func() {
        c.lastCollection = collectionStart
    }()

    sortedModules := c.getSortedModules()
    var collectionErrors []error

    for i := 0; i < len(sortedModules); i += c.batchSize {
        // Calculate batch end
        end := i + c.batchSize
        if end > len(sortedModules) {
            end = len(sortedModules)
        }

        // Process current batch
        batch := sortedModules[i:end]
        log.Printf("Processing batch %d-%d of %d modules", 
            i+1, end, len(sortedModules))

        if err := c.processBatch(ctx, batch); err != nil {
            collectionErrors = append(collectionErrors, err)
            log.Printf("Batch error: %v", err)
        }

        // Wait between batches unless this is the last batch
        if end < len(sortedModules) {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(c.batchInterval):
                // Continue to next batch
            }
        }
    }

    if len(collectionErrors) > 0 {
        return fmt.Errorf("collection errors: %v", collectionErrors)
    }

    return nil
}

func (c *Collector) processBatch(ctx context.Context, batch []ModuleCollector) error {
    for _, module := range batch {
        moduleCtx, cancel := context.WithTimeout(ctx, c.config.Collector.TimeoutPerModule)
        
        log.Printf("Collecting metrics for module: %s (priority: %d)", 
            module.Name(), module.Priority())

        start := time.Now()
        err := module.Collect(moduleCtx)
        duration := time.Since(start)

        cancel()

        if err != nil {
            log.Printf("Error collecting %s metrics: %v (took %v)", 
                module.Name(), err, duration)
            continue
        }

        log.Printf("Successfully collected %s metrics in %v", 
            module.Name(), duration)
    }

    return nil
}

func (c *Collector) getSortedModules() []ModuleCollector {
    sorted := make([]ModuleCollector, len(c.modules))
    copy(sorted, c.modules)

    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].Priority() > sorted[j].Priority()
    })

    return sorted
}

func (c *Collector) updateStats(success bool, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.stats.TotalCollections++
    if success {
        c.stats.SuccessfulBatches++
        c.stats.LastSuccessTime = time.Now()
    } else {
        c.stats.FailedBatches++
        c.stats.LastError = err
    }
}

func (c *Collector) GetStatus() map[string]interface{} {
    c.mu.RLock()
    defer c.mu.RUnlock()

    return map[string]interface{}{
        "running":           true,
        "start_time":        c.startTime,
        "last_collection":   c.lastCollection,
        "total_collections": c.stats.TotalCollections,
        "successful_batches": c.stats.SuccessfulBatches,
        "failed_batches":    c.stats.FailedBatches,
        "last_error":        c.stats.LastError,
        "last_success":      c.stats.LastSuccessTime,
        "batch_size":        c.batchSize,
        "batch_interval":    c.batchInterval,
        "module_count":      len(c.modules),
    }
}
