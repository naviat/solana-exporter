package collector

import (
    "context"
    "time"
)

// Collector defines the interface that all metric collectors must implement
type Collector interface {
    // Name returns the unique name of the collector
    Name() string

    // Collect gathers metrics and updates the Prometheus metrics
    Collect(ctx context.Context) error
}

// StoppableCollector is an optional interface that collectors can implement
// if they need cleanup when stopping
type StoppableCollector interface {
    Collector
    Stop() error
}

// CollectorConfig contains configuration for collectors
type CollectorConfig struct {
    Interval    time.Duration
    Labels      map[string]string
    MetricsPath string
}
