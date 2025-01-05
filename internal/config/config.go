package config

import (
    "fmt"
    "os"
    "time"
    "gopkg.in/yaml.v3"
)

type Config struct {
    RPC struct {
        // RPC node endpoints
        Endpoint          string        `yaml:"endpoint"`
        ReferenceEndpoint string        `yaml:"reference_endpoint"`
        
        // Connection settings
        Timeout          time.Duration `yaml:"timeout"`
        MaxRetries       int           `yaml:"max_retries"`
        RetryBackoff     time.Duration `yaml:"retry_backoff"`
        
        // Cache settings
        VersionCacheDuration time.Duration `yaml:"version_cache_duration"`
        HealthCacheDuration  time.Duration `yaml:"health_cache_duration"`
        
        // Rate limiting
        MaxRequestsPerSecond int `yaml:"max_requests_per_second"`
    } `yaml:"rpc"`

    WebSocket struct {
        // WS settings
        Enabled  bool   `yaml:"enabled"`
        Endpoint string `yaml:"endpoint"`
        
        // Connection settings
        MaxConnections  int           `yaml:"max_connections"`
        ReconnectDelay  time.Duration `yaml:"reconnect_delay"`
        
        // Subscription settings
        SubscriptionBatchSize int           `yaml:"subscription_batch_size"`
        SubscriptionTimeout   time.Duration `yaml:"subscription_timeout"`
    } `yaml:"websocket"`

    Metrics struct {
        // Collection intervals
        SlotMetricsInterval   time.Duration     `yaml:"slot_metrics_interval"`
        HealthMetricsInterval time.Duration     `yaml:"health_metrics_interval"`
        WSMetricsInterval     time.Duration     `yaml:"ws_metrics_interval"`
        
        // Labels
        DefaultLabels map[string]string `yaml:"default_labels"`
    } `yaml:"metrics"`
}

// LoadConfig reads the configuration file and returns a Config struct
func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }

    if err := config.validate(); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }

    config.setDefaults()
    return &config, nil
}

// validate checks if the configuration is valid
func (c *Config) validate() error {
    if c.RPC.Endpoint == "" {
        return fmt.Errorf("RPC endpoint is required")
    }

    if c.RPC.ReferenceEndpoint == "" {
        return fmt.Errorf("reference RPC endpoint is required")
    }

    if c.WebSocket.Enabled && c.WebSocket.Endpoint == "" {
        return fmt.Errorf("WebSocket endpoint is required when WebSocket is enabled")
    }

    return nil
}

// setDefaults sets default values for optional configuration fields
func (c *Config) setDefaults() {
    // RPC defaults
    if c.RPC.Timeout == 0 {
        c.RPC.Timeout = 30 * time.Second
    }
    if c.RPC.MaxRetries == 0 {
        c.RPC.MaxRetries = 3
    }
    if c.RPC.RetryBackoff == 0 {
        c.RPC.RetryBackoff = time.Second
    }
    if c.RPC.VersionCacheDuration == 0 {
        c.RPC.VersionCacheDuration = 5 * time.Minute
    }
    if c.RPC.HealthCacheDuration == 0 {
        c.RPC.HealthCacheDuration = time.Minute
    }
    if c.RPC.MaxRequestsPerSecond == 0 {
        c.RPC.MaxRequestsPerSecond = 50
    }

    // WebSocket defaults
    if c.WebSocket.MaxConnections == 0 {
        c.WebSocket.MaxConnections = 5
    }
    if c.WebSocket.ReconnectDelay == 0 {
        c.WebSocket.ReconnectDelay = 5 * time.Second
    }
    if c.WebSocket.SubscriptionBatchSize == 0 {
        c.WebSocket.SubscriptionBatchSize = 3
    }
    if c.WebSocket.SubscriptionTimeout == 0 {
        c.WebSocket.SubscriptionTimeout = 10 * time.Second
    }

    // Metrics defaults
    if c.Metrics.SlotMetricsInterval == 0 {
        c.Metrics.SlotMetricsInterval = 2 * time.Second
    }
    if c.Metrics.HealthMetricsInterval == 0 {
        c.Metrics.HealthMetricsInterval = 15 * time.Second
    }
    if c.Metrics.WSMetricsInterval == 0 {
        c.Metrics.WSMetricsInterval = 5 * time.Second
    }

    // Initialize default labels map if not set
    if c.Metrics.DefaultLabels == nil {
        c.Metrics.DefaultLabels = make(map[string]string)
    }

    // Set default labels if not present
    if _, ok := c.Metrics.DefaultLabels["exporter_version"]; !ok {
        c.Metrics.DefaultLabels["exporter_version"] = "1.0.0"
    }
}
