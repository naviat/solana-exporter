package performance

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

func TestCollector_TransactionMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup performance samples mock
    sampleData := []PerformanceSample{
        {
            Slot:             100,
            NumTransactions:  1000,
            NumSlots:        10,
            SamplePeriodSecs: 10,
        },
        {
            Slot:             110,
            NumTransactions:  2000,
            NumSlots:        10,
            SamplePeriodSecs: 10,
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

    // Setup transaction stats mock
    mockClient.On("Call",
        mock.Anything,
        "getTransactionCount",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*TransactionStats)) = TransactionStats{
            Confirmed:   800,
            Failed:     200,
            ProcessTime: 0.5,
        }
    }, nil)

    err := collector.collectTransactionMetrics(context.Background())
    assert.NoError(t, err)

    // Verify metrics
    baseLabels := []string{"test-node:8899"}
    assert.Equal(t, float64(150), metrics.TxThroughput.WithLabelValues(baseLabels...).Get()) // (100 + 200) / 2
    assert.Equal(t, float64(80), metrics.TxSuccessRate.WithLabelValues(baseLabels...).Get()) // 800/(800+200) * 100
}

func TestCollector_BlockMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        "getBlockProduction",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*BlockProductionStats)) = BlockProductionStats{
            NumBlocks:    100,
            NumSlots:     120,
            SamplePeriod: 60,
        }
    }, nil)

    mockClient.On("Call",
        mock.Anything,
        "getBlockHeight",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        *(result.(*uint64)) = 1000
    }, nil)

    err := collector.collectBlockMetrics(context.Background())
    assert.NoError(t, err)

    baseLabels := []string{"test-node:8899"}
    assert.Equal(t, float64(1000), metrics.BlockHeight.WithLabelValues(baseLabels...).Get())
}

func TestCollector_SlotMetrics(t *testing.T) {
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

    baseLabels := []string{"test-node:8899"}
    assert.Equal(t, float64(100), metrics.CurrentSlot.WithLabelValues(baseLabels...).Get())
    assert.Equal(t, float64(105), metrics.NetworkSlot.WithLabelValues(baseLabels...).Get())
    assert.Equal(t, float64(5), metrics.SlotDiff.WithLabelValues(baseLabels...).Get())
}

func TestCollector_ConfirmationMetrics(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        "getConfirmationTimeStats",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        stats := result.(*struct {
            Mean   float64 `json:"mean"`
            Stddev float64 `json:"stddev"`
        })
        stats.Mean = 0.5
        stats.Stddev = 0.1
    }, nil)

    err := collector.collectConfirmationMetrics(context.Background())
    assert.NoError(t, err)

    // Verify confirmation time metrics were recorded
    baseLabels := []string{"test-node:8899"}
    metric := metrics.TxConfirmationTime.WithLabelValues(baseLabels...)
    assert.NotNil(t, metric)
}

func TestCollector_Collect(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup mocks for all calls
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
}

func TestCollector_WithCancelledContext(t *testing.T) {
    _, _, collector := setupTest()

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    err := collector.Collect(ctx)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context canceled")
}

func TestCollector_WithTimeout(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup slow response
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

func TestCollector_ConcurrentCollection(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup expectations
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    // Run multiple collections concurrently
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

    // Check for any errors
    for err := range errs {
        assert.NoError(t, err)
    }
}

func BenchmarkCollector_Collect(b *testing.B) {
    mockClient, _, collector := setupTest()

    // Setup expectations
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

func TestCollector_ErrorHandling(t *testing.T) {
    tests := []struct {
        name        string
        method      string
        setupMock   func(*MockClient)
        expectError bool
    }{
        {
            name:   "transaction metrics error",
            method: "getRecentPerformanceSamples",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getRecentPerformanceSamples",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, assert.AnError)
            },
            expectError: true,
        },
        {
            name:   "block metrics error",
            method: "getBlockProduction",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getBlockProduction",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, assert.AnError)
            },
            expectError: true,
        },
        {
            name:   "slot metrics error",
            method: "getSlot",
            setupMock: func(m *MockClient) {
                m.On("Call",
                    mock.Anything,
                    "getSlot",
                    mock.Anything,
                    mock.Anything,
                ).Return(nil, assert.AnError)
            },
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient, _, collector := setupTest()
            tt.setupMock(mockClient)

            err := collector.Collect(context.Background())
            if tt.expectError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.method)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestCollector_MetricsValidity(t *testing.T) {
    _, metrics, _ := setupTest()

    // Verify all metrics are properly initialized
    assert.NotNil(t, metrics.TxThroughput)
    assert.NotNil(t, metrics.TxSuccessRate)
    assert.NotNil(t, metrics.TxErrorRate)
    assert.NotNil(t, metrics.BlockHeight)
    assert.NotNil(t, metrics.CurrentSlot)
    assert.NotNil(t, metrics.NetworkSlot)
    assert.NotNil(t, metrics.SlotDiff)
    assert.NotNil(t, metrics.TxConfirmationTime)
}

func TestCollector_LabelValidation(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup basic successful response
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(nil, nil)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)

    // Verify labels are correctly set
    expectedLabels := []string{"test-node:8899"}
    
    metric := metrics.TxThroughput.WithLabelValues(expectedLabels...)
    assert.NotNil(t, metric)

    metric = metrics.CurrentSlot.WithLabelValues(expectedLabels...)
    assert.NotNil(t, metric)
}

func TestCollector_Name(t *testing.T) {
    _, _, collector := setupTest()
    assert.Equal(t, "performance", collector.Name())
}
