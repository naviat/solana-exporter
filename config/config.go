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

    System struct {
        EnableDiskMetrics    bool `yaml:"enable_disk_metrics"`
        EnableCPUMetrics     bool `yaml:"enable_cpu_metrics"`
        EnableMemoryMetrics  bool `yaml:"enable_memory_metrics"`
        EnableNetworkMetrics bool `yaml:"enable_network_metrics"`
    } `yaml:"system"`
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
