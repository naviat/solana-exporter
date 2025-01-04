package main

import (
    "context"
    "flag"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "solana-exporter/internal/collector"
    "solana-exporter/internal/config"
    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

var (
    configFile = flag.String("config", "config.yaml", "Path to configuration file")
    listenAddr = flag.String("web.listen-address", ":9104", "Address to listen on for telemetry")
)

func main() {
    flag.Parse()

    // Load configuration
    cfg, err := config.LoadConfig(*configFile)
    if err != nil {
        log.Fatalf("Error loading config: %v", err)
    }

    // Create Prometheus registry
    reg := prometheus.NewRegistry()

    // Initialize metrics
    m := metrics.NewMetrics(reg)

    // Initialize Solana clients
    localClient := solana.NewClient(
        cfg.RPC.Endpoint,
        cfg.WebSocket.Endpoint,
        cfg.RPC.Timeout,
        cfg.RPC.MaxRetries,
        cfg.RPC.MaxRequestsPerSecond,
    )
    defer localClient.Close()

    referenceClient := solana.NewClient(
        cfg.RPC.ReferenceEndpoint,
        "",  // No WebSocket for reference client
        cfg.RPC.Timeout,
        cfg.RPC.MaxRetries,
        cfg.RPC.MaxRequestsPerSecond,
    )
    defer referenceClient.Close()

    // Initialize WebSocket connection if enabled
    if cfg.WebSocket.Enabled {
        ctx, cancel := context.WithTimeout(context.Background(), cfg.RPC.Timeout)
        defer cancel()
        
        if err := localClient.ConnectWebSocket(ctx); err != nil {
            log.Printf("Warning: Failed to establish WebSocket connection: %v", err)
        }
    }

    // Initialize collectors
    collectors := []collector.Collector{
        collector.NewRPCCollector(localClient, referenceClient, m, cfg.Metrics.DefaultLabels),
    }

    if cfg.WebSocket.Enabled {
        wsCollector := collector.NewWSCollector(localClient, m, cfg.Metrics.DefaultLabels)
        collectors = append(collectors, wsCollector)
    }

    // Create context that listens for the interrupt signal
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    // Start collectors
    for _, c := range collectors {
        go runCollector(ctx, c, cfg)
    }

    // Set up HTTP server
    http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
        EnableOpenMetrics: true,
    }))

    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    })

    // Create server with proper timeouts
    server := &http.Server{
        Addr: *listenAddr,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start server
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Printf("Error starting server: %v", err)
        }
    }()

    log.Printf("Server started on %s", *listenAddr)

    // Wait for interrupt signal
    <-ctx.Done()
    log.Println("Shutting down...")

    // Shutdown with timeout
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(shutdownCtx); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }

    // Stop collectors
    for _, c := range collectors {
        if stopper, ok := c.(collector.StoppableCollector); ok {
            if err := stopper.Stop(); err != nil {
                log.Printf("Error stopping collector %s: %v", c.Name(), err)
            }
        }
    }
}

func runCollector(ctx context.Context, c collector.Collector, cfg *config.Config) {
    // Determine collection interval based on collector type
    var interval time.Duration
    switch c.Name() {
    case "rpc":
        interval = cfg.Metrics.SlotMetricsInterval
    case "websocket":
        interval = cfg.Metrics.WSMetricsInterval
    default:
        interval = cfg.Metrics.HealthMetricsInterval
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := c.Collect(ctx); err != nil {
                log.Printf("Error collecting %s metrics: %v", c.Name(), err)
            }
        }
    }
}
