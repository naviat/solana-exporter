package config

import (
    "fmt"
    "os"
    "time"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Server struct {
        Port int    `yaml:"port"`
        Host string `yaml:"host"`
    } `yaml:"server"`

    RPC struct {
        // HTTP RPC settings
        Timeout      time.Duration `yaml:"timeout"`
        MaxRetries   int          `yaml:"max_retries"`
        RetryBackoff time.Duration `yaml:"retry_backoff"`
        
        // WebSocket settings
        WSPort       int          `yaml:"ws_port"`
        WSTimeout    time.Duration `yaml:"ws_timeout"`
        
        // Rate limiting settings
        RequestsPerSecond int `yaml:"requests_per_second"`
    } `yaml:"rpc"`

    Collector struct {
        // Basic collection settings
        Interval          time.Duration `yaml:"interval"`
        TimeoutPerModule  time.Duration `yaml:"timeout_per_module"`
        
        // Batch processing settings
        BatchSize         int           `yaml:"batch_size"`
        BatchInterval     time.Duration `yaml:"batch_interval"`
        
        // Collection priorities
        RPCPriority      int           `yaml:"rpc_priority"`
        CachePriority    int           `yaml:"cache_priority"`
        WSPriority       int           `yaml:"ws_priority"`
    } `yaml:"collector"`
}

func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file: %w", err)
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }

    if err := config.setDefaults(); err != nil {
        return nil, fmt.Errorf("set defaults: %w", err)
    }

    if err := config.validate(); err != nil {
        return nil, fmt.Errorf("validate config: %w", err)
    }

    return &config, nil
}

func (c *Config) setDefaults() error {
    // Server defaults
    if c.Server.Port == 0 {
        c.Server.Port = 8080
    }
    if c.Server.Host == "" {
        c.Server.Host = "0.0.0.0"
    }

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
    if c.RPC.RequestsPerSecond == 0 {
        c.RPC.RequestsPerSecond = 50
    }

    // Collector defaults
    if c.Collector.Interval == 0 {
        c.Collector.Interval = 15 * time.Second
    }
    if c.Collector.TimeoutPerModule == 0 {
        c.Collector.TimeoutPerModule = 10 * time.Second
    }
    if c.Collector.BatchSize == 0 {
        c.Collector.BatchSize = 3
    }
    if c.Collector.BatchInterval == 0 {
        c.Collector.BatchInterval = 2 * time.Second
    }

    // Priority defaults
    if c.Collector.RPCPriority == 0 {
        c.Collector.RPCPriority = 100
    }
    if c.Collector.CachePriority == 0 {
        c.Collector.CachePriority = 80
    }
    if c.Collector.WSPriority == 0 {
        c.Collector.WSPriority = 60
    }

    return nil
}

func (c *Config) validate() error {
    if c.Server.Port < 0 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", c.Server.Port)
    }
    if c.Collector.BatchSize < 1 {
        return fmt.Errorf("batch size must be at least 1")
    }
    if c.Collector.BatchInterval < time.Second {
        return fmt.Errorf("batch interval must be at least 1 second")
    }
    return nil
}
