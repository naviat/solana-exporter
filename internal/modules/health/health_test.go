package health

import (
    "context"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
)

// MockClient implements a mock RPC client for testing
type MockClient struct {
    mock.Mock
}

func (m *MockClient) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    args := m.Called(ctx, method, params, result)
    
    // If result pointer is provided and mock has data, copy it
    if result != nil && args.Get(0) != nil {
        switch v := result.(type) {
        case *HealthResponse:
            *v = args.Get(0).(HealthResponse)
        case *VersionResponse:
            *v = args.Get(0).(VersionResponse)
        }
    }
    
    return args.Error(1)
}

func setupTest() (*MockClient, *metrics.Metrics, *Collector) {
    mockClient := new(MockClient)
    reg := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(reg)
    collector := NewCollector(mockClient, metrics)
    
    return mockClient, metrics, collector
}

func TestCollector_CollectHealth(t *testing.T) {
    tests := []struct {
        name           string
        healthResponse HealthResponse
        expectError    bool
        expectedValue  float64
    }{
        {
            name:           "healthy node",
            healthResponse: HealthResponse{Status: "ok"},
            expectError:    false,
            expectedValue:  1,
        },
        {
            name:           "unhealthy node",
            healthResponse: HealthResponse{Status: "error"},
            expectError:    false,
            expectedValue:  0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient, metrics, collector := setupTest()

            // Setup mock expectation
            mockClient.On("Call", 
                mock.Anything, 
                "getHealth",
                mock.Anything,
                mock.Anything,
            ).Return(tt.healthResponse, nil)

            // Call collector
            err := collector.collectHealth(context.Background())

            // Assert expectations
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }

            // Check metrics
            if tt.healthResponse.Status == "ok" {
                assert.Equal(t, tt.expectedValue, metrics.NodeHealth.WithLabelValues("ok").Get())
                assert.Equal(t, 0.0, metrics.NodeHealth.WithLabelValues("error").Get())
            } else {
                assert.Equal(t, 0.0, metrics.NodeHealth.WithLabelValues("ok").Get())
                assert.Equal(t, 1.0, metrics.NodeHealth.WithLabelValues("error").Get())
            }

            mockClient.AssertExpectations(t)
        })
    }
}

func TestCollector_CollectVersion(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    expectedVersion := VersionResponse{
        SolanaCore: "1.14.10",
        FeatureSet: 123456789,
    }

    // Setup mock expectation
    mockClient.On("Call",
        mock.Anything,
        "getVersion",
        mock.Anything,
        mock.Anything,
    ).Return(expectedVersion, nil)

    // Call collector
    err := collector.collectVersion(context.Background())

    // Assert expectations
    assert.NoError(t, err)
    assert.Equal(t, 1.0, metrics.NodeVersion.WithLabelValues(
        expectedVersion.SolanaCore,
        "123456789",
    ).Get())

    mockClient.AssertExpectations(t)
}

func TestCollector_CollectSystemHealth(t *testing.T) {
    _, metrics, collector := setupTest()

    // Call collector
    err := collector.collectSystemHealth(context.Background())

    // Assert expectations
    assert.NoError(t, err)

    // Verify that metrics were collected
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("heap").Get(), 0.0)
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("stack").Get(), 0.0)
    assert.Greater(t, metrics.MemoryUsage.WithLabelValues("system").Get(), 0.0)
    assert.Greater(t, metrics.GoroutineCount.Get(), 0.0)
    assert.Greater(t, metrics.LastRestartTime.Get(), float64(time.Now().Add(-1*time.Minute).Unix()))
}

func TestCollector_Collect(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mock expectations for all calls
    mockClient.On("Call",
        mock.Anything,
        "getHealth",
        mock.Anything,
        mock.Anything,
    ).Return(HealthResponse{Status: "ok"}, nil)

    mockClient.On("Call",
        mock.Anything,
        "getVersion",
        mock.Anything,
        mock.Anything,
    ).Return(VersionResponse{
        SolanaCore: "1.14.10",
        FeatureSet: 123456789,
    }, nil)

    // Call main collect function
    err := collector.Collect(context.Background())

    // Assert expectations
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}

func TestCollector_Name(t *testing.T) {
    _, _, collector := setupTest()
    assert.Equal(t, "health", collector.Name())
}

func TestCollector_WithCancelledContext(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Create cancelled context
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    // Call collector with cancelled context
    err := collector.Collect(ctx)

    // Should return error due to cancelled context
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
}
