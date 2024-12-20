package rpc

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
    return args.Error(0)
}

func setupTest() (*MockClient, *metrics.Metrics, *Collector) {
    mockClient := new(MockClient)
    reg := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(reg)
    collector := NewCollector(mockClient, metrics)
    return mockClient, metrics, collector
}

func TestRPCCollector_MeasureLatency(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup mock
    mockClient.On("Call", 
        mock.Anything, 
        "getSlot",
        mock.Anything,
        mock.Anything,
    ).Return(nil)

    // Measure latency
    err := collector.measureRPCLatency(context.Background(), "getSlot")
    
    assert.NoError(t, err)
    
    // Verify metrics
    assert.Equal(t, float64(1), metrics.RPCRequests.WithLabelValues("getSlot").Get())
    
    // Verify latency was recorded
    metric := metrics.RPCLatency.WithLabelValues("getSlot")
    assert.NotNil(t, metric)
}

func TestRPCCollector_RequestSizes(t *testing.T) {
    _, metrics, collector := setupTest()

    err := collector.measureRequestSizes(context.Background())
    
    assert.NoError(t, err)
    
    // Verify that request sizes were recorded
    assert.Greater(t, metrics.RPCRequestSize.Get(), float64(0))
}

func TestRPCCollector_InFlightRequests(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup mock
    mockClient.On("Call", 
        mock.Anything, 
        "getRecentPerformanceSamples",
        mock.Anything,
        mock.Anything,
    ).Return(nil)

    err := collector.trackInFlightRequests(context.Background())
    
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}

func TestRPCCollector_Collect(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mocks for all methods
    mockClient.On("Call", 
        mock.Anything, 
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil)

    err := collector.Collect(context.Background())
    
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}
