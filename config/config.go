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

    Metrics struct {
        Port int    `yaml:"port"`
        Path string `yaml:"path"`
    } `yaml:"metrics"`

    Collector struct {
        Interval time.Duration `yaml:"interval"`
    } `yaml:"collector"`

    System struct {
        EnableDiskMetrics bool `yaml:"enable_disk_metrics"`
        EnableCPUMetrics  bool `yaml:"enable_cpu_metrics"`
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
    if config.Metrics.Port == 0 {
        config.Metrics.Port = 9090
    }
    if config.Metrics.Path == "" {
        config.Metrics.Path = "/metrics"
    }

    return &config, nil
}