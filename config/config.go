package config

import (
    "fmt"
    "os"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Server struct {
        Port    int           `yaml:"port"`
        Host    string        `yaml:"host"`
        Path    string        `yaml:"path"`
        Timeout time.Duration `yaml:"timeout"`
    } `yaml:"server"`

    RPC struct {
        Endpoint     string        `yaml:"endpoint"`
        Timeout      time.Duration `yaml:"timeout"`
        MaxRetries   int           `yaml:"max_retries"`
        RetryBackoff time.Duration `yaml:"retry_backoff"`
    } `yaml:"rpc"`

    Collector struct {
        Interval          time.Duration `yaml:"interval"`
        TimeoutPerModule  time.Duration `yaml:"timeout_per_module"`
        ConcurrentModules int          `yaml:"concurrent_modules"`
        BatchSize         int          `yaml:"batch_size"`
    } `yaml:"collector"`

    Metrics struct {
        Labels            map[string]string   `yaml:"labels"`
        ExcludedMetrics   []string           `yaml:"excluded_metrics"`
        HistogramBuckets  map[string][]float64 `yaml:"histogram_buckets"`
        DefaultBuckets    []float64           `yaml:"default_buckets"`
    } `yaml:"metrics"`

    System struct {
        EnableCPUMetrics     bool `yaml:"enable_cpu_metrics"`
        EnableMemoryMetrics  bool `yaml:"enable_memory_metrics"`
        EnableDiskMetrics    bool `yaml:"enable_disk_metrics"`
        EnableNetworkMetrics bool `yaml:"enable_network_metrics"`
    } `yaml:"system"`

    Logging struct {
        Level   string `yaml:"level"`
        Format  string `yaml:"format"`
        File    string `yaml:"file"`
    } `yaml:"logging"`
}

// LoadConfig reads and parses the configuration file
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("error reading config file: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("error parsing config file: %w", err)
    }

    // Set default values
    if err := config.setDefaults(); err != nil {
        return nil, fmt.Errorf("error setting defaults: %w", err)
    }

    // Validate configuration
    if err := config.validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return &config, nil
}

func (c *Config) setDefaults() error {
    // Server defaults
    if c.Server.Port == 0 {
        c.Server.Port = 8080
    }
    if c.Server.Path == "" {
        c.Server.Path = "/solana/"
    }
    if c.Server.Timeout == 0 {
        c.Server.Timeout = 30 * time.Second
    }

    // RPC defaults
    if c.RPC.Timeout == 0 {
        c.RPC.Timeout = 10 * time.Second
    }
    if c.RPC.MaxRetries == 0 {
        c.RPC.MaxRetries = 3
    }
    if c.RPC.RetryBackoff == 0 {
        c.RPC.RetryBackoff = time.Second
    }

    // Collector defaults
    if c.Collector.Interval == 0 {
        c.Collector.Interval = 15 * time.Second
    }
    if c.Collector.TimeoutPerModule == 0 {
        c.Collector.TimeoutPerModule = 5 * time.Second
    }
    if c.Collector.ConcurrentModules == 0 {
        c.Collector.ConcurrentModules = 4
    }
    if c.Collector.BatchSize == 0 {
        c.Collector.BatchSize = 100
    }

    // Metrics defaults
    if c.Metrics.Labels == nil {
        c.Metrics.Labels = make(map[string]string)
    }
    if len(c.Metrics.DefaultBuckets) == 0 {
        c.Metrics.DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
    }

    return nil
}

func (c *Config) validate() error {
    if c.Server.Port < 0 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", c.Server.Port)
    }

    if c.RPC.Timeout < time.Second {
        return fmt.Errorf("rpc timeout too short: %v", c.RPC.Timeout)
    }

    if c.Collector.Interval < time.Second {
        return fmt.Errorf("collector interval too short: %v", c.Collector.Interval)
    }

    if c.Collector.TimeoutPerModule >= c.Collector.Interval {
        return fmt.Errorf("module timeout (%v) must be less than collection interval (%v)",
            c.Collector.TimeoutPerModule, c.Collector.Interval)
    }

    return nil
}