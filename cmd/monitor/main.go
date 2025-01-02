package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"

    "solana-rpc-monitor/config"
    "solana-rpc-monitor/internal/collector"
    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

func main() {
    configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
    flag.Parse()

    // Load configuration
    cfg, err := config.LoadConfig(*configPath)
    if err != nil {
        log.Fatalf("Error loading config: %v", err)
    }

    // Initialize metrics registry
    reg := prometheus.NewRegistry()
    metricsCollector := metrics.NewMetrics(reg)

    // Create HTTP handler for metrics
    http.HandleFunc("/solana/", func(w http.ResponseWriter, r *http.Request) {
        // Extract node information from request parameters
        nodeAddress := r.URL.Query().Get("node_address")
        nodeScheme := r.URL.Query().Get("node_scheme")

        if nodeAddress == "" {
            http.Error(w, "node_address parameter is required", http.StatusBadRequest)
            return
        }

        if nodeScheme == "" {
            nodeScheme = "http"
        }

        // Create labels from request parameters
        labels := map[string]string{
            "node_address": nodeAddress,
            "org":         r.URL.Query().Get("org"),
            "node_type":   r.URL.Query().Get("node_type"),
            "release":     r.URL.Query().Get("release"),
        }

        // Create RPC client for this node with both HTTP and WS endpoints
        endpoint := fmt.Sprintf("%s://%s", nodeScheme, nodeAddress)
        client := solana.NewClient(endpoint, cfg.RPC.WSPort, cfg.RPC.Timeout, labels)
        if client == nil {
            http.Error(w, "Failed to create client", http.StatusInternalServerError)
            return
        }

        // Create collector for this node
        nodeCollector := collector.NewCollector(client, metricsCollector, cfg, labels)
        
        // Collect metrics
        if err := nodeCollector.Collect(r.Context()); err != nil {
            http.Error(w, fmt.Sprintf("Error collecting metrics: %v", err), http.StatusInternalServerError)
            return
        }

        promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, r)
    })

    // Health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        sigCh := make(chan os.Signal, 1)
        signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
        <-sigCh
        log.Println("Received shutdown signal")
        cancel()
    }()

    // Start server
    server := &http.Server{
        Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
        Handler: http.DefaultServeMux,
    }

    go func() {
        <-ctx.Done()
        if err := server.Shutdown(context.Background()); err != nil {
            log.Printf("Error shutting down server: %v", err)
        }
    }()

    log.Printf("Starting Solana RPC exporter on port %d", cfg.Server.Port)
    if err := server.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("Error starting server: %v", err)
    }
}