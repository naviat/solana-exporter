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

func TestPerformanceCollector_TransactionMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    sampleData := []PerformanceSample{
        {
            NumTransactions:   1000,
            NumSlots:         10,
            SamplePeriodSecs: 10,
            Slot:            100,
        },
        {
            NumTransactions:   2000,
            NumSlots:         10,
            SamplePeriodSecs: 10,
            Slot:            110,
        },
    }

    mockClient.On("Call",
        mock.Anything,
        "getRecentPerformanceSamples",
        []interface{}{5},
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*[]PerformanceSample)) = sampleData
    }, nil)

    err := collector.collectTransactionMetrics(context.Background())
    assert.NoError(t, err)

    // Verify TPS calculations
    labels := []string{"test-node:8799", "test-org", "rpc"}
    assert.Equal(t, float64(150), metrics.TxThroughput.WithLabelValues(labels...).Get()) // (100 + 200) / 2
}

func TestPerformanceCollector_SlotMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Mock current slot
    mockClient.On("Call",
        mock.Anything,
        "getSlot",
        nil,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*uint64)) = 100
    }, nil)

    // Mock network slot
    mockClient.On("Call",
        mock.Anything,
        "getSlot",
        []interface{}{map[string]string{"commitment": "finalized"}},
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*uint64)) = 105
    }, nil)

    err := collector.collectSlotMetrics(context.Background())
    assert.NoError(t, err)

    labels := []string{"test-node:8799", "test-org", "rpc"}
    assert.Equal(t, float64(100), metrics.CurrentSlot.WithLabelValues(labels...).Get())
    assert.Equal(t, float64(105), metrics.NetworkSlot.WithLabelValues(labels...).Get())
    assert.Equal(t, float64(5), metrics.SlotDiff.WithLabelValues(labels...).Get())
}

func TestPerformanceCollector_ErrorHandling(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Test transaction metrics error
    mockClient.On("Call",
        mock.Anything,
        "getRecentPerformanceSamples",
        mock.Anything,
        mock.Anything,
    ).Return(nil, assert.AnError)

    err := collector.collectTransactionMetrics(context.Background())
    assert.Error(t, err)
}

func TestPerformanceCollector_EmptySamples(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Return empty samples
    mockClient.On("Call",
        mock.Anything,
        "getRecentPerformanceSamples",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*[]PerformanceSample)) = []PerformanceSample{}
    }, nil)

    err := collector.collectTransactionMetrics(context.Background())
    assert.NoError(t, err)

    // Verify metrics weren't updated
    labels := []string{"test-node:8799", "test-org", "rpc"}
    assert.Equal(t, float64(0), metrics.TxThroughput.WithLabelValues(labels...).Get())
}

func TestPerformanceCollector_ConcurrentCollection(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mocks for concurrent calls
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        switch v := result.(type) {
        case *[]PerformanceSample:
            *v = []PerformanceSample{{
                NumTransactions:   1000,
                NumSlots:         10,
                SamplePeriodSecs: 10,
                Slot:            100,
            }}
        case *uint64:
            *v = 100
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

func TestPerformanceCollector_ContextTimeout(t *testing.T) {
    _, _, collector := setupTest()

    ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
    defer cancel()

    time.Sleep(2 * time.Millisecond)
    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}
