package health

import (
    "context"
    "testing"
    "time"
    "sync"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"

    "solana-rpc-monitor/internal/metrics"
)

type MockClient struct {
    mock.Mock
}

func (m *MockClient) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    args := m.Called(ctx, method, params, result)
    if fn, ok := args.Get(0).(func(interface{})); ok {
        fn(result)
    }
    return args.Error(1)
}

func setupTest() (*MockClient, *metrics.Metrics, *Collector) {
    mockClient := new(MockClient)
    registry := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(registry)
    
    labels := map[string]string{
        "node_address": "test-node:8899",
        "org":          "test-org",
        "node_type":    "rpc",
    }

    collector := NewCollector(mockClient, metrics, labels)
    return mockClient, metrics, collector
}

func TestCollector_CollectHealth(t *testing.T) {
    tests := []struct {
        name           string
        healthStatus   string
        expectError    bool
        expectedValue  float64
        expectedLabels []string
    }{
        {
            name:           "healthy node",
            healthStatus:   "ok",
            expectError:    false,
            expectedValue:  1,
            expectedLabels: []string{"test-node:8899", "ok"},
        },
        {
            name:           "unhealthy node",
            healthStatus:   "error",
            expectError:    false,
            expectedValue:  0,
            expectedLabels: []string{"test-node:8899", "ok"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient, metrics, collector := setupTest()

            mockClient.On("Call",
                mock.Anything,
                "getHealth",
                mock.Anything,
                mock.Anything,
            ).Return(func(result interface{}) {
                *(result.(*HealthResponse)) = HealthResponse{Status: tt.healthStatus}
            }, nil)

            err := collector.collectHealth(context.Background())

            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedValue, 
                    metrics.NodeHealth.WithLabelValues(tt.expectedLabels...).Get())
            }
        })
    }
}

func TestCollector_CollectVersion(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    expectedVersion := VersionResponse{
        SolanaCore: "1.14.10",
        FeatureSet: 123456789,
    }

    mockClient.On("Call",
        mock.Anything,
        "getVersion",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*VersionResponse)) = expectedVersion
    }, nil)

    err := collector.collectVersion(context.Background())
    assert.NoError(t, err)

    baseLabels := []string{"test-node:8899", expectedVersion.SolanaCore, "123456789"}
    assert.Equal(t, float64(1), metrics.NodeVersion.WithLabelValues(baseLabels...).Get())
}

func TestCollector_CollectNodeInfo(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup mocks
    identity := NodeIdentity{Identity: "test-pubkey"}
    nodes := []ClusterNode{
        {
            Pubkey:  "test-pubkey",
            RPC:     "localhost:8899",
            Version: "1.14.10",
        },
    }

    mockClient.On("Call",
        mock.Anything,
        "getIdentity",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*NodeIdentity)) = identity
    }, nil)

    mockClient.On("Call",
        mock.Anything,
        "getClusterNodes",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*[]ClusterNode)) = nodes
    }, nil)

    err := collector.collectNodeInfo(context.Background())
    assert.NoError(t, err)

    baseLabels := []string{"test-node:8899", "rpc_available"}
    assert.Equal(t, float64(1), metrics.NodeHealth.WithLabelValues(baseLabels...).Get())
}

func TestCollector_CollectSystemHealth(t *testing.T) {
    _, metrics, collector := setupTest()

    err := collector.collectSystemHealth(context.Background())
    assert.NoError(t, err)

    baseLabels := []string{"test-node:8899"}
    
    // Verify memory metrics
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(baseLabels, "heap")...).Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(baseLabels, "stack")...).Get(), float64(0))
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues(append(baseLabels, "system")...).Get(), float64(0))
    
    // Verify goroutine count
    assert.Greater(t, metrics.GoroutineCount.WithLabelValues(baseLabels...).Get(), float64(0))
    
    // Verify last restart time
    assert.Greater(t, metrics.LastRestartTime.WithLabelValues(baseLabels...).Get(), float64(0))
}

func TestCollector_Collect(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mocks for all method calls
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
}

func TestCollector_ConcurrentCollection(t *testing.T) {
    mockClient, _, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    var wg sync.WaitGroup
    errs := make(chan error, 5)

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := collector.Collect(context.Background()); err != nil {
                errs <- err
            }
        }()
    }

    wg.Wait()
    close(errs)

    for err := range errs {
        assert.NoError(t, err)
    }
}

func TestCollector_ContextCancellation(t *testing.T) {
    _, _, collector := setupTest()

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
}

func TestCollector_Timeout(t *testing.T) {
    mockClient, _, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Run(func(args mock.Arguments) {
        time.Sleep(100 * time.Millisecond)
    }).Return(nil, nil)

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestCollector_ErrorHandling(t *testing.T) {
    tests := []struct {
        name        string
        method      string
        setupMock   func(*MockClient)
        expectError bool
        errorMsg    string
    }{
        {
            name:   "health check error",
            method: "getHealth",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getHealth",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, fmt.Errorf("health check failed"))
            },
            expectError: true,
            errorMsg:    "health check failed",
        },
        {
            name:   "version check error",
            method: "getVersion",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getVersion",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, fmt.Errorf("version check failed"))
            },
            expectError: true,
            errorMsg:    "version check failed",
        },
        {
            name:   "node info error",
            method: "getIdentity",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getIdentity",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, fmt.Errorf("identity check failed"))
            },
            expectError: true,
            errorMsg:    "failed to get node identity",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient, _, collector := setupTest()
            tt.setupMock(mockClient)

            err := collector.Collect(context.Background())
            if tt.expectError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestCollector_MetricsValidity(t *testing.T) {
    _, metrics, _ := setupTest()

    // Verify all metrics are properly initialized
    assert.NotNil(t, metrics.NodeHealth)
    assert.NotNil(t, metrics.NodeVersion)
    assert.NotNil(t, metrics.MemoryUsage)
    assert.NotNil(t, metrics.GoroutineCount)
    assert.NotNil(t, metrics.LastRestartTime)
}

func BenchmarkCollector_Collect(b *testing.B) {
    mockClient, _, collector := setupTest()

    // Setup successful responses
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        collector.Collect(ctx)
    }
}

func TestCollector_LabelValidation(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup basic successful response
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        switch v := result.(type) {
        case *HealthResponse:
            *v = HealthResponse{Status: "ok"}
        case *VersionResponse:
            *v = VersionResponse{SolanaCore: "1.14.10", FeatureSet: 123456789}
        }
    }, nil)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)

    // Verify labels are correctly set
    baseLabels := []string{"test-node:8899"}
    
    // Check health metrics labels
    healthMetric := metrics.NodeHealth.WithLabelValues(append(baseLabels, "ok")...)
    assert.NotNil(t, healthMetric)

    // Check version metrics labels
    versionMetric := metrics.NodeVersion.WithLabelValues(append(baseLabels, "1.14.10", "123456789")...)
    assert.NotNil(t, versionMetric)

    // Check memory metrics labels
    memoryMetric := metrics.MemoryUsage.WithLabelValues(append(baseLabels, "heap")...)
    assert.NotNil(t, memoryMetric)
}

func TestCollector_SystemMetricsRange(t *testing.T) {
    _, metrics, collector := setupTest()

    err := collector.collectSystemHealth(context.Background())
    assert.NoError(t, err)

    baseLabels := []string{"test-node:8899"}

    // Memory metrics should be positive
    assert.GreaterOrEqual(t, 
        metrics.MemoryUsage.WithLabelValues(append(baseLabels, "heap")...).Get(),
        float64(0),
    )
    assert.GreaterOrEqual(t, 
        metrics.MemoryUsage.WithLabelValues(append(baseLabels, "stack")...).Get(),
        float64(0),
    )
    assert.GreaterOrEqual(t, 
        metrics.MemoryUsage.WithLabelValues(append(baseLabels, "system")...).Get(),
        float64(0),
    )

    // Goroutine count should be positive
    assert.GreaterOrEqual(t, 
        metrics.GoroutineCount.WithLabelValues(baseLabels...).Get(),
        float64(1),
    )

    // Last restart time should be reasonable
    now := float64(time.Now().Unix())
    lastRestart := metrics.LastRestartTime.WithLabelValues(baseLabels...).Get()
    assert.Less(t, now-lastRestart, float64(60)) // Within last minute
}

func TestCollector_Name(t *testing.T) {
    _, _, collector := setupTest()
    assert.Equal(t, "health", collector.Name())
}