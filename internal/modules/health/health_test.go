package health

import (
    "context"
    "testing"
    "time"

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
        "node_address": "test-node:8799",
        "org":          "test-org",
        "node_type":    "rpc",
    }

    collector := NewCollector(mockClient, metrics, labels)
    return mockClient, metrics, collector
}

func TestHealthCollector_HealthCheck(t *testing.T) {
    mockClient, metrics, collector := setupTest()

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
            expectedLabels: []string{"test-node:8799", "test-org", "rpc", "ok"},
        },
        {
            name:           "unhealthy node",
            healthStatus:   "error",
            expectError:    false,
            expectedValue:  0,
            expectedLabels: []string{"test-node:8799", "test-org", "rpc", "ok"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mock
            mockClient.On("Call",
                mock.Anything,
                "getHealth",
                mock.Anything,
                mock.Anything,
            ).Return(func(result interface{}) {
                *(result.(*HealthResponse)) = HealthResponse{Status: tt.healthStatus}
            }, nil).Once()

            err := collector.collectHealth(context.Background())

            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedValue, metrics.NodeHealth.WithLabelValues(tt.expectedLabels...).Get())
            }

            mockClient.AssertExpectations(t)
        })
    }
}

func TestHealthCollector_VersionCheck(t *testing.T) {
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

    labels := []string{
        "test-node:8799",
        "test-org",
        "rpc",
        expectedVersion.SolanaCore,
        "123456789",
    }
    assert.Equal(t, float64(1), metrics.NodeVersion.WithLabelValues(labels...).Get())
}

func TestHealthCollector_ErrorHandling(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Test health check error
    mockClient.On("Call",
        mock.Anything,
        "getHealth",
        mock.Anything,
        mock.Anything,
    ).Return(nil, assert.AnError).Once()

    err := collector.collectHealth(context.Background())
    assert.Error(t, err)

    labels := []string{"test-node:8799", "test-org", "rpc", "error"}
    assert.Equal(t, float64(0), metrics.NodeHealth.WithLabelValues(labels...).Get())
}

func TestHealthCollector_Concurrent(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mock for concurrent calls
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

    // Run multiple collections concurrently
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := collector.Collect(context.Background())
            assert.NoError(t, err)
        }()
    }
    wg.Wait()
}

func TestHealthCollector_ContextHandling(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Test context cancellation
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")

    // Test context timeout
    ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
    defer cancel()
    time.Sleep(2 * time.Millisecond)
    err = collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}
