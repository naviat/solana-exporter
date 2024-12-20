package performance

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
    reg := prometheus.NewRegistry()
    metrics := metrics.NewMetrics(reg)
    collector := NewCollector(mockClient, metrics)
    return mockClient, metrics, collector
}

func TestCollectTransactionMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    sampleData := []PerformanceSample{
        {
            Slot:             100,
            NumTransactions:  1000,
            NumSlots:        10,
            SamplePeriodSecs: 10,
        },
    }

    mockClient.On("Call",
        mock.Anything,
        "getRecentPerformanceSamples",
        mock.Anything,
        mock.Anything,
    ).Return(func(interface{}) {
        *(result.(*[]PerformanceSample)) = sampleData
    }, nil)

    err := collector.collectTransactionMetrics(context.Background())
    
    assert.NoError(t, err)
    assert.Equal(t, 100.0, metrics.TxThroughput.Get()) // 1000 tx / 10 sec = 100 TPS
    mockClient.AssertExpectations(t)
}

func TestCollectBlockProcessingMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        "getBlockTime",
        mock.Anything,
        mock.Anything,
    ).Return(func(interface{}) {
        *(result.(*struct{ Average float64 })) = struct{ Average float64 }{Average: 0.5}
    }, nil)

    err := collector.collectBlockProcessingMetrics(context.Background())
    
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}

func TestCollectConfirmationMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        "getConfirmationTime",
        mock.Anything,
        mock.Anything,
    ).Return(func(interface{}) {
        *(result.(*struct {
            Mean   float64
            Stddev float64
        })) = struct {
            Mean   float64
            Stddev float64
        }{Mean: 0.3, Stddev: 0.1}
    }, nil)

    err := collector.collectConfirmationMetrics(context.Background())
    
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}

func TestCollector_Collect(t *testing.T) {
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
