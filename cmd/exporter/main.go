// cmd/exporter/main.go
package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    "sync"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/gorilla/mux"

    "solana-exporter/internal/collector"
    "solana-exporter/config"
    "solana-exporter/internal/metrics"
    "solana-exporter/pkg/solana"
)

type ExporterService struct {
    config     *config.Config
    registry   *prometheus.Registry
    metrics    *metrics.Metrics
    server     *http.Server
    collectors map[string]*collector.Collector
    mu         sync.RWMutex
}

func main() {
    // Parse command line flags
    configPath := flag.String("config", "config/config.yaml", "Path to configuration file")
    logPath := flag.String("log", "", "Path to log file (defaults to stdout)")
    flag.Parse()

    // Configure logging
    if *logPath != "" {
        logFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
        if err != nil {
            log.Fatalf("Failed to open log file: %v", err)
        }
        defer logFile.Close()
        log.SetOutput(logFile)
    }

    // Load configuration
    cfg, err := config.LoadConfig(*configPath)
    if err != nil {
        log.Fatalf("Error loading config: %v", err)
    }

    // Create and start the exporter service
    service := NewExporterService(cfg)
    if err := service.Start(); err != nil {
        log.Fatalf("Failed to start service: %v", err)
    }
}

func NewExporterService(cfg *config.Config) *ExporterService {
    return &ExporterService{
        config:     cfg,
        registry:   prometheus.NewRegistry(),
        collectors: make(map[string]*collector.Collector),
    }
}

func (s *ExporterService) Start() error {
    // Initialize metrics
    s.metrics = metrics.NewMetrics(s.registry)

    // Create router and set up routes
    router := mux.NewRouter()
    s.setupRoutes(router)

    // Create HTTP server
    s.server = &http.Server{
        Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
        Handler:      router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Set up graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Start server in a goroutine
    go func() {
        log.Printf("Starting Solana exporter on %s", s.server.Addr)
        if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("HTTP server error: %v", err)
        }
    }()

    // Wait for shutdown signal
    <-sigChan
    log.Println("Shutdown signal received")

    // Initiate graceful shutdown
    return s.Shutdown(ctx)
}

func (s *ExporterService) setupRoutes(router *mux.Router) {
    // Metrics endpoint with node parameter
    router.HandleFunc("/solana/metrics", s.handleMetrics).Methods("GET")
    
    // Health check endpoint
    router.HandleFunc("/health", s.handleHealth).Methods("GET")
    
    // Status endpoint
    router.HandleFunc("/status", s.handleStatus).Methods("GET")
}

func (s *ExporterService) handleMetrics(w http.ResponseWriter, r *http.Request) {
    nodeAddress := r.URL.Query().Get("node_address")
    if nodeAddress == "" {
        http.Error(w, "node_address parameter is required", http.StatusBadRequest)
        return
    }

    // Validate node address format
    if nodeAddress == "mainnet-beta-rpc.example.com" {
        http.Error(w, "Please provide a valid RPC node address", http.StatusBadRequest)
        return
    }

    // Create collection context with timeout
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
    defer cancel()

    // Get or create collector
    collector, err := s.getCollector(nodeAddress, r.URL.Query())
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to initialize collector: %v", err), http.StatusInternalServerError)
        return
    }

    // Collect metrics with proper error handling
    if err := collector.Collect(ctx); err != nil {
        log.Printf("Error collecting metrics for %s: %v", nodeAddress, err)
        // Continue to serve metrics even if collection partially failed
    }

    promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{
        EnableOpenMetrics: true,
    }).ServeHTTP(w, r)
}

func (s *ExporterService) getCollector(nodeAddress string, params map[string][]string) (*collector.Collector, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if collector already exists
    if collector, exists := s.collectors[nodeAddress]; exists {
        return collector, nil
    }

    // Create labels from request parameters
    labels := map[string]string{
        "node_address": nodeAddress,
        "node_type":    getFirstValue(params["node_type"]), // Using helper function instead of .Get()
    }

    // Create RPC client
    endpoint := fmt.Sprintf("http://%s", nodeAddress)
    client := solana.NewClient(endpoint, s.config.RPC.WSPort, s.config.RPC.Timeout, s.config.RPC.MaxRetries)
    if client == nil {
        return nil, fmt.Errorf("failed to create client")
    }

    // Create new collector
    collector := collector.NewCollector(client, s.metrics, s.config, labels)
    s.collectors[nodeAddress] = collector

    return collector, nil
}

func (s *ExporterService) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, `{"status":"healthy"}`)
}

func (s *ExporterService) handleStatus(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    status := map[string]interface{}{
        "start_time":     time.Now().Format(time.RFC3339),
        "node_count":     len(s.collectors),
        "configuration":  s.config.Server,
        "collector_info": make(map[string]interface{}),
    }

    // Collect status from each collector
    for node, collector := range s.collectors {
        status["collector_info"].(map[string]interface{})[node] = collector.GetStatus()
    }
    s.mu.RUnlock()

    // Write response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

// Helper function to safely get the first value from a string slice
func getFirstValue(values []string) string {
    if len(values) > 0 {
        return values[0]
    }
    return ""
}

func (s *ExporterService) Shutdown(ctx context.Context) error {
    // Create a timeout context for shutdown
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Stop all collectors
    s.mu.Lock()
    for _, collector := range s.collectors {
        if err := collector.Stop(); err != nil {
            log.Printf("Error stopping collector: %v", err)
        }
    }
    s.mu.Unlock()

    // Shutdown HTTP server
    if err := s.server.Shutdown(shutdownCtx); err != nil {
        return fmt.Errorf("error shutting down HTTP server: %w", err)
    }

    log.Println("Service shutdown completed")
    return nil
}
