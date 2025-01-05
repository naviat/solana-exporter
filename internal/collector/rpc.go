package collector

import (
    "context"
    "fmt"
    "log"
    "runtime"
    "sync"
    "syscall"
    "time"
    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

// collectedMetrics holds all metrics for consistent logging
type collectedMetrics struct {
    mu             sync.Mutex
    slot           uint64
    referenceSlot  uint64
    slotBehind     uint64
    blockTime      int64
    epoch          uint64
    epochProgress  float64
    isHealthy      bool
    version        string
    memoryUsed     uint64
    openFDs        uint64
    maxFDs         uint64
    goroutines     int
}

type RPCCollector struct {
    localClient     *solana.Client
    referenceClient *solana.Client
    metrics         *metrics.Metrics
    nodeLabels      map[string]string
    cache           *metricCache
}

type metricCache struct {
    mu             sync.RWMutex
    versionInfo    string
    versionTime    time.Time
    healthStatus   bool
    healthTime     time.Time
    cacheTimeout   time.Duration
}

func NewRPCCollector(localClient, referenceClient *solana.Client, metrics *metrics.Metrics, labels map[string]string) *RPCCollector {
    if labels == nil {
        labels = make(map[string]string)
    }
    labels["endpoint"] = localClient.GetEndpoint()
    return &RPCCollector{
        localClient:     localClient,
        referenceClient: referenceClient,
        metrics:         metrics,
        nodeLabels:      labels,
        cache: &metricCache{
            cacheTimeout: 5 * time.Minute,
        },
    }
}

func (c *RPCCollector) Name() string {
    return "rpc"
}

func (c *RPCCollector) Collect(ctx context.Context) error {
    metrics := &collectedMetrics{}
    endpoint := c.nodeLabels["endpoint"]

    // Always collect system metrics first as they don't depend on RPC
    if err := c.collectSystemMetrics(ctx, metrics); err != nil {
        log.Printf("[%s] Error collecting system metrics: %v", endpoint, err)
    }

    // Create wait group and error channel for concurrent collection
    var wg sync.WaitGroup
    errCh := make(chan error, 4) // Buffer size matches number of concurrent collectors

    // Launch collectors concurrently
    collectors := []struct {
        name string
        fn   func(context.Context, *collectedMetrics) error
    }{
        {"node status", c.collectNodeStatus},
        {"slot metrics", c.collectSlotMetrics},
        {"epoch info", c.collectEpochInfo},
        {"block metrics", c.collectBlockMetrics},
    }

    for _, collector := range collectors {
        wg.Add(1)
        go func(name string, fn func(context.Context, *collectedMetrics) error) {
            defer wg.Done()
            if err := fn(ctx, metrics); err != nil {
                errCh <- fmt.Errorf("%s: %w", name, err)
                log.Printf("[%s] Error collecting %s: %v", endpoint, name, err)
            }
        }(collector.name, collector.fn)
    }

    // Wait for all collectors to complete
    wg.Wait()
    close(errCh)

    // Gather any errors that occurred
    var collectionErrors []error
    for err := range errCh {
        collectionErrors = append(collectionErrors, err)
    }

    // Log system metrics even if there are errors
    log.Printf("[%s] System Stats | Goroutines=%d, Memory=%d bytes, FDs=%d/%d",
        endpoint,
        metrics.goroutines,
        metrics.memoryUsed,
        metrics.openFDs,
        metrics.maxFDs)

    // Log Solana metrics if available
    if metrics.slot > 0 {
        log.Printf("[%s] Solana Stats | Slot=%d (Behind=%d) | Epoch=%d (Progress=%.2f%%) | Version=%s | Healthy=%v",
            endpoint,
            metrics.slot,
            metrics.slotBehind,
            metrics.epoch,
            metrics.epochProgress,
            metrics.version,
            metrics.isHealthy)
    }

    if len(collectionErrors) > 0 {
        log.Printf("[%s] Collection errors: %v", endpoint, collectionErrors)
        return fmt.Errorf("multiple collection errors occurred: %v", collectionErrors)
    }

    return nil
}
// Run sequence to get metrics............
// func (c *RPCCollector) Collect(ctx context.Context) error {
//     metrics := &collectedMetrics{}
//     endpoint := c.nodeLabels["endpoint"]

//     // Always collect system metrics first as they don't depend on RPC
//     if err := c.collectSystemMetrics(ctx, metrics); err != nil {
//         log.Printf("[%s] Error collecting system metrics: %v", endpoint, err)
//     }

//     // Try to collect Solana metrics
//     var collectionErrors []error

//     // Collect in sequence to avoid overwhelming the RPC node
//     if err := c.collectNodeStatus(ctx, metrics); err != nil {
//         collectionErrors = append(collectionErrors, fmt.Errorf("node status: %w", err))
//     }

//     if err := c.collectSlotMetrics(ctx, metrics); err != nil {
//         collectionErrors = append(collectionErrors, fmt.Errorf("slot metrics: %w", err))
//     }

//     if err := c.collectEpochInfo(ctx, metrics); err != nil {
//         collectionErrors = append(collectionErrors, fmt.Errorf("epoch info: %w", err))
//     }

//     if err := c.collectBlockMetrics(ctx, metrics); err != nil {
//         collectionErrors = append(collectionErrors, fmt.Errorf("block metrics: %w", err))
//     }

//     // Log system metrics even if there are errors
//     log.Printf("[%s] System Stats | Goroutines=%d, Memory=%d bytes, FDs=%d/%d",
//         endpoint,
//         metrics.goroutines,
//         metrics.memoryUsed,
//         metrics.openFDs,
//         metrics.maxFDs)

//     // Log Solana metrics if available
//     if metrics.slot > 0 {
//         log.Printf("[%s] Solana Stats | Slot=%d (Behind=%d) | Epoch=%d (Progress=%.2f%%) | Version=%s | Healthy=%v",
//             endpoint,
//             metrics.slot,
//             metrics.slotBehind,
//             metrics.epoch,
//             metrics.epochProgress,
//             metrics.version,
//             metrics.isHealthy)
//     }

//     if len(collectionErrors) > 0 {
//         log.Printf("[%s] Collection errors: %v", endpoint, collectionErrors)
//         return fmt.Errorf("collection errors occurred")
//     }

//     return nil
// }

func (c *RPCCollector) collectSlotMetrics(ctx context.Context, metrics *collectedMetrics) error {
    endpoint := c.nodeLabels["endpoint"]
    var (
        wg    sync.WaitGroup
        errCh = make(chan error, 2)
    )

    // Get local node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := c.localClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &metrics.slot)

        if err != nil {
            log.Printf("[%s] Failed to get local slot: %v", endpoint, err)
            errCh <- err
            return
        }

        metrics.mu.Lock()
        c.metrics.CurrentSlot.WithLabelValues(
            endpoint,
            "finalized",
        ).Set(float64(metrics.slot))
        metrics.mu.Unlock()
    }()

    // Get reference node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := c.referenceClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &metrics.referenceSlot)

        if err != nil {
            log.Printf("[%s] Failed to get reference slot: %v", endpoint, err)
            errCh <- err
            return
        }

        metrics.mu.Lock()
        c.metrics.NetworkSlot.WithLabelValues(endpoint).Set(float64(metrics.referenceSlot))

        if metrics.slot > 0 && metrics.referenceSlot > 0 {
            slotDiff := metrics.referenceSlot - metrics.slot
            if slotDiff >= 0 {
                metrics.slotBehind = slotDiff
                c.metrics.SlotBehind.WithLabelValues(endpoint).Set(float64(slotDiff))
            }
        }
        metrics.mu.Unlock()
    }()

    wg.Wait()
    close(errCh)

    var errs []error
    for err := range errCh {
        if err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("slot metrics: %v", errs)
    }

    return nil
}

func (c *RPCCollector) collectBlockMetrics(ctx context.Context, metrics *collectedMetrics) error {
    endpoint := c.nodeLabels["endpoint"]

    err := c.localClient.Call(ctx, "getSlot", []interface{}{
        map[string]string{"commitment": "finalized"},
    }, &metrics.slot)

    if err != nil {
        return fmt.Errorf("get slot: %w", err)
    }

    c.metrics.BlockHeight.WithLabelValues(endpoint).Set(float64(metrics.slot))

    // Get block time
    err = c.localClient.Call(ctx, "getBlockTime", []interface{}{metrics.slot}, &metrics.blockTime)
    if err != nil {
        return fmt.Errorf("get block time: %w", err)
    }

    if metrics.blockTime > 0 {
        timeSinceBlock := time.Now().Unix() - metrics.blockTime
        c.metrics.BlockTime.WithLabelValues(endpoint).Set(float64(timeSinceBlock))
    }

    return nil
}

func (c *RPCCollector) collectEpochInfo(ctx context.Context, metrics *collectedMetrics) error {
    endpoint := c.nodeLabels["endpoint"]

    type EpochInfo struct {
        AbsoluteSlot  uint64 `json:"absoluteSlot"`
        BlockHeight   uint64 `json:"blockHeight"`
        Epoch         uint64 `json:"epoch"`
        SlotIndex     uint64 `json:"slotIndex"`
        SlotsInEpoch  uint64 `json:"slotsInEpoch"`
    }

    var epochInfo EpochInfo
    err := c.localClient.Call(ctx, "getEpochInfo", []interface{}{
        map[string]string{"commitment": "finalized"},
    }, &epochInfo)

    if err != nil {
        return fmt.Errorf("get epoch info: %w", err)
    }

    metrics.mu.Lock()
    metrics.epoch = epochInfo.Epoch
    metrics.epochProgress = (float64(epochInfo.SlotIndex) / float64(epochInfo.SlotsInEpoch)) * 100
    metrics.mu.Unlock()

    c.metrics.EpochInfo.WithLabelValues(endpoint).Set(float64(epochInfo.Epoch))
    c.metrics.SlotOffset.WithLabelValues(endpoint).Set(float64(epochInfo.SlotIndex))

    slotsRemaining := epochInfo.SlotsInEpoch - epochInfo.SlotIndex
    c.metrics.SlotsRemaining.WithLabelValues(endpoint).Set(float64(slotsRemaining))
    c.metrics.EpochProgress.WithLabelValues(endpoint).Set(metrics.epochProgress)

    epochStartSlot := epochInfo.AbsoluteSlot - epochInfo.SlotIndex
    c.metrics.ConfirmedEpochFirstSlot.WithLabelValues(endpoint).Set(float64(epochStartSlot))
    c.metrics.ConfirmedEpochLastSlot.WithLabelValues(endpoint).Set(float64(epochStartSlot + epochInfo.SlotsInEpoch))
    c.metrics.ConfirmedEpochNumber.WithLabelValues(endpoint).Set(float64(epochInfo.Epoch))

    return nil
}

func (c *RPCCollector) collectNodeStatus(ctx context.Context, metrics *collectedMetrics) error {
    endpoint := c.nodeLabels["endpoint"]

    var healthStatus string
    err := c.localClient.Call(ctx, "getHealth", nil, &healthStatus)
    if err != nil {
        metrics.mu.Lock()
        metrics.isHealthy = false
        metrics.mu.Unlock()
        c.metrics.NodeHealth.WithLabelValues(endpoint).Set(0)
        return fmt.Errorf("get health: %w", err)
    }

    metrics.mu.Lock()
    metrics.isHealthy = healthStatus == "ok"
    metrics.mu.Unlock()

    healthValue := 0.0
    if metrics.isHealthy {
        healthValue = 1.0
    }
    c.metrics.NodeHealth.WithLabelValues(endpoint).Set(healthValue)

    var versionInfo struct {
        SolanaCore string `json:"solana-core"`
    }
    
    if err := c.localClient.Call(ctx, "getVersion", nil, &versionInfo); err != nil {
        return fmt.Errorf("get version: %w", err)
    }

    metrics.mu.Lock()
    metrics.version = versionInfo.SolanaCore
    metrics.mu.Unlock()

    c.metrics.NodeVersion.WithLabelValues(
        endpoint,
        versionInfo.SolanaCore,
    ).Set(1)

    return nil
}

func (c *RPCCollector) collectSystemMetrics(ctx context.Context, metrics *collectedMetrics) error {
    endpoint := c.nodeLabels["endpoint"]

    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    metrics.mu.Lock()
    metrics.memoryUsed = m.Alloc
    metrics.goroutines = runtime.NumGoroutine()
    metrics.mu.Unlock()

    c.metrics.SystemMemoryUsed.WithLabelValues(endpoint).Set(float64(m.Alloc))
    c.metrics.SystemHeapAlloc.WithLabelValues(endpoint).Set(float64(m.HeapAlloc))
    c.metrics.SystemGoroutines.WithLabelValues(endpoint).Set(float64(runtime.NumGoroutine()))
    c.metrics.SystemThreads.WithLabelValues(endpoint).Set(float64(runtime.NumCPU()))

    var rLimit syscall.Rlimit
    err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
    if err != nil {
        return fmt.Errorf("get fd limits: %w", err)
    }

    metrics.mu.Lock()
    metrics.maxFDs = uint64(rLimit.Max)
    metrics.openFDs = uint64(rLimit.Cur)
    metrics.mu.Unlock()

    c.metrics.SystemMaxFDs.WithLabelValues(endpoint).Set(float64(rLimit.Max))
    c.metrics.SystemOpenFDs.WithLabelValues(endpoint).Set(float64(rLimit.Cur))

    return nil
}

func (c *RPCCollector) Stop() error {
    return nil
}
