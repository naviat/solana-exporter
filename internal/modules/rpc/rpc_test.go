package rpc

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"

    "solana-rpc-monitor/internal/metrics"
    "solana-rpc-monitor/pkg/solana"
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

func TestRPCCollector_MeasureLatency(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup expectations
    mockClient.On("Call", 
        mock.Anything, 
        "getSlot",
        mock.Anything,
        mock.Anything,
    ).Return(func(interface{}) {
        time.Sleep(10 * time.Millisecond) // Simulate latency
    }, nil)

    // Test latency measurement
    err := collector.measureRPCLatency(context.Background(), "getSlot")
    assert.NoError(t, err)

    // Verify metrics
    assert.Equal(t, float64(1), metrics.RPCRequests.WithLabelValues("test-node:8799", "test-org", "rpc", "getSlot").Get())
    
    // Verify latency was recorded (should be around 10ms)
    metric := metrics.RPCLatency.WithLabelValues("test-node:8799", "test-org", "rpc", "getSlot")
    assert.NotNil(t, metric)
}

func TestRPCCollector_RequestSizes(t *testing.T) {
    _, metrics, collector := setupTest()

    err := collector.measureRequestSizes(context.Background())
    assert.NoError(t, err)

    // Verify that request sizes were recorded
    metric := metrics.RPCRequestSize.WithLabelValues("test-node:8799", "test-org", "rpc")
    assert.NotNil(t, metric)
}

func TestRPCCollector_InFlightRequests(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup mock
    mockClient.On("Call",
        mock.Anything,
        "getResourceConsumption",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*struct{ CurrentRequests int })) = struct{ CurrentRequests int }{
            CurrentRequests: 42,
        }
    }, nil)

    err := collector.trackInFlightRequests(context.Background())
    assert.NoError(t, err)

    // Verify metrics
    assert.Equal(t, float64(42), metrics.RPCInFlight.WithLabelValues("test-node:8799", "test-org", "rpc").Get())
}

func TestRPCCollector_Error(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup expectations for error case
    mockClient.On("Call",
        mock.Anything,
        "getSlot",
        mock.Anything,
        mock.Anything,
    ).Return(nil, assert.AnError)

    err := collector.measureRPCLatency(context.Background(), "getSlot")
    assert.Error(t, err)

    // Verify error metrics
    assert.Equal(t, float64(1), metrics.RPCErrors.WithLabelValues(
        "test-node:8799", "test-org", "rpc", "getSlot", "request_failed").Get())
}

func TestRPCCollector_MultipleMetrics(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup expectations for multiple methods
    for _, method := range monitoredMethods {
        mockClient.On("Call",
            mock.Anything,
            method,
            mock.Anything,
            mock.Anything,
        ).Return(nil, nil)
    }

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}
