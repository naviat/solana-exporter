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
    // Add default endpoint label if not present
    if _, ok := labels["endpoint"]; !ok {
        labels["endpoint"] = localClient.GetEndpoint()
    }
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
    var wg sync.WaitGroup
    errCh := make(chan error, 4)

    collectors := []struct {
        name string
        fn   func(context.Context) error
    }{
        {"slot", c.collectSlotMetrics},
        {"block", c.collectBlockMetrics},
        {"epoch", c.collectEpochInfo},
        {"system", c.collectSystemMetrics},
    }

    for _, collector := range collectors {
        wg.Add(1)
        go func(name string, fn func(context.Context) error) {
            defer wg.Done()
            if err := fn(ctx); err != nil {
                log.Printf("Error collecting %s metrics: %v", name, err)
                errCh <- fmt.Errorf("%s: %w", name, err)
            }
        }(collector.name, collector.fn)
    }

    // Collect node status metrics
    if err := c.collectNodeStatus(ctx); err != nil {
        errCh <- fmt.Errorf("node status: %w", err)
    }

    wg.Wait()
    close(errCh)

    var errs []error
    for err := range errCh {
        if err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("collection errors: %v", errs)
    }

    return nil
}

func (c *RPCCollector) collectEpochInfo(ctx context.Context) error {
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

    c.metrics.EpochInfo.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(epochInfo.Epoch))
    
    c.metrics.SlotOffset.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(epochInfo.SlotIndex))

    slotsRemaining := epochInfo.SlotsInEpoch - epochInfo.SlotIndex
    c.metrics.SlotsRemaining.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(slotsRemaining))

    progress := (float64(epochInfo.SlotIndex) / float64(epochInfo.SlotsInEpoch)) * 100
    c.metrics.EpochProgress.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(progress)

    // Record epoch boundaries
    epochStartSlot := epochInfo.AbsoluteSlot - epochInfo.SlotIndex
    c.metrics.ConfirmedEpochFirstSlot.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(epochStartSlot))
    c.metrics.ConfirmedEpochLastSlot.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(epochStartSlot + epochInfo.SlotsInEpoch))
    c.metrics.ConfirmedEpochNumber.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(epochInfo.Epoch))

    return nil
}

func (c *RPCCollector) collectSlotMetrics(ctx context.Context) error {
    var (
        localSlot uint64
        refSlot   uint64
        wg        sync.WaitGroup
        errCh     = make(chan error, 2)
    )

    // Get local node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := c.localClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &localSlot)

        if err != nil {
            errCh <- err
            return
        }

        c.metrics.CurrentSlot.WithLabelValues(
            c.nodeLabels["endpoint"],
            "finalized",
        ).Set(float64(localSlot))
        log.Printf("Local slot: %d", localSlot)
    }()

    // Get reference node slot
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := c.referenceClient.Call(ctx, "getSlot", []interface{}{
            map[string]string{"commitment": "finalized"},
        }, &refSlot)

        if err != nil {
            errCh <- err
            return
        }

        c.metrics.NetworkSlot.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(float64(refSlot))

        if localSlot > 0 && refSlot > 0 {
            slotDiff := refSlot - localSlot
            if slotDiff >= 0 {
                c.metrics.SlotBehind.WithLabelValues(c.nodeLabels["endpoint"]).
                    Set(float64(slotDiff))
                log.Printf("Reference slot: %d, Slot behind: %d", refSlot, slotDiff)
            }
        }
    }()

    wg.Wait()
    close(errCh)

    for err := range errCh {
        if err != nil {
            return fmt.Errorf("slot metrics: %w", err)
        }
    }

    return nil
}

func (c *RPCCollector) collectBlockMetrics(ctx context.Context) error {
    // Get current slot
    var slot uint64
    err := c.localClient.Call(ctx, "getSlot", []interface{}{
        map[string]string{"commitment": "finalized"},
    }, &slot)

    if err != nil {
        return fmt.Errorf("get slot: %w", err)
    }

    c.metrics.BlockHeight.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(slot))

    // Get block time for current slot
    var blockTime int64
    err = c.localClient.Call(ctx, "getBlockTime", []interface{}{slot}, &blockTime)
    if err != nil {
        return fmt.Errorf("get block time: %w", err)
    }

    if blockTime > 0 {
        timeSinceBlock := time.Now().Unix() - blockTime
        c.metrics.BlockTime.WithLabelValues(c.nodeLabels["endpoint"]).
            Set(float64(timeSinceBlock))
        log.Printf("Current block - Slot: %d, Time since block: %d seconds", slot, timeSinceBlock)
    }

    return nil
}

func (c *RPCCollector) collectNodeStatus(ctx context.Context) error {
    // Get node health
    var healthStatus string
    err := c.localClient.Call(ctx, "getHealth", nil, &healthStatus)
    if err != nil {
        c.metrics.NodeHealth.WithLabelValues(c.nodeLabels["endpoint"]).Set(0)
        log.Printf("Node health check failed: %v", err)
        return fmt.Errorf("get health: %w", err)
    }

    isHealthy := healthStatus == "ok"
    healthValue := 0.0
    if isHealthy {
        healthValue = 1.0
    }
    c.metrics.NodeHealth.WithLabelValues(c.nodeLabels["endpoint"]).Set(healthValue)

    // Get node version
    var versionInfo struct {
        SolanaCore string `json:"solana-core"`
    }
    
    if err := c.localClient.Call(ctx, "getVersion", nil, &versionInfo); err != nil {
        log.Printf("Failed to get node version: %v", err)
        return fmt.Errorf("get version: %w", err)
    }

    c.metrics.NodeVersion.WithLabelValues(
        c.nodeLabels["endpoint"],
        versionInfo.SolanaCore,
    ).Set(1)

    return nil
}

func (c *RPCCollector) collectSystemMetrics(ctx context.Context) error {
    // Collect memory stats
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    c.metrics.SystemMemoryTotal.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(m.Sys))
    c.metrics.SystemMemoryUsed.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(m.Alloc))
    c.metrics.SystemHeapAlloc.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(m.HeapAlloc))

    // Collect goroutine and thread count
    c.metrics.SystemGoroutines.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(runtime.NumGoroutine()))
    c.metrics.SystemThreads.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(runtime.NumCPU()))

    // Collect file descriptor stats
    var rLimit syscall.Rlimit
    err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
    if err != nil {
        return fmt.Errorf("get fd limits: %w", err)
    }

    c.metrics.SystemMaxFDs.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(rLimit.Max))
    c.metrics.SystemOpenFDs.WithLabelValues(c.nodeLabels["endpoint"]).
        Set(float64(rLimit.Cur))

    // Collect GC stats
    c.metrics.SystemGCDuration.WithLabelValues(c.nodeLabels["endpoint"]).
        Observe(float64(m.PauseTotalNs) / float64(time.Second))

    return nil
}

// Stop implements the StoppableCollector interface
func (c *RPCCollector) Stop() error {
    // Nothing to clean up for RPC collector
    return nil
}
