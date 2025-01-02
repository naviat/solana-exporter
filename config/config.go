package config

import (
    "os"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    RPC struct {
        Endpoint string        `yaml:"endpoint"`
        Timeout  time.Duration `yaml:"timeout"`
        MaxRetries   int          `yaml:"max_retries"`
        RetryBackoff time.Duration `yaml:"retry_backoff"`
        WSPort       int          `yaml:"ws_port"`
        WSEnabled    bool         `yaml:"ws_enabled"`
        HealthCheckEnabled bool          `yaml:"health_check_enabled"`
    } `yaml:"rpc"`

    Server struct {
        Port int           `yaml:"port"`
        Host string        `yaml:"host"`
        Path string        `yaml:"path"`
        Timeout time.Duration `yaml:"timeout"`
    } `yaml:"server"`

    Collector struct {
        Interval         time.Duration `yaml:"interval"`
        TimeoutPerModule time.Duration `yaml:"timeout_per_module"`
        ConcurrentModules int          `yaml:"concurrent_modules"`
    } `yaml:"collector"`
}

func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var config Config
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    // Set defaults if not specified
    if config.RPC.Timeout == 0 {
        config.RPC.Timeout = 30 * time.Second
    }
    if config.RPC.MaxRetries == 0 {
        config.RPC.MaxRetries = 3
    }
    if config.RPC.RetryBackoff == 0 {
        config.RPC.RetryBackoff = time.Second
    }
    if config.RPC.WSPort == 0 {
        config.RPC.WSPort = 8800 // Default WebSocket port
    }
    if config.Collector.Interval == 0 {
        config.Collector.Interval = 15 * time.Second
    }
    if config.Collector.TimeoutPerModule == 0 {
        config.Collector.TimeoutPerModule = 5 * time.Second
    }
    if config.Collector.ConcurrentModules == 0 {
        config.Collector.ConcurrentModules = 4
    }
    if config.Server.Port == 0 {
        config.Server.Port = 8080
    }
    if config.Server.Host == "" {
        config.Server.Host = "0.0.0.0"
    }
    if config.Server.Path == "" {
        config.Server.Path = "/solana/"
    }
    if config.Server.Timeout == 0 {
        config.Server.Timeout = 30 * time.Second
    }

    return &config, nil
}
