package rpc

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"

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
        "node_address": "test-node:8899",
        "org":          "test-org",
        "node_type":    "rpc",
    }

    collector := NewCollector(mockClient, metrics, labels)
    return mockClient, metrics, collector
}

func TestCollector_Name(t *testing.T) {
    _, _, collector := setupTest()
    assert.Equal(t, "rpc", collector.Name())
}

func TestCollector_MeasureRPCLatency(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        response       interface{}
        expectError    bool
        expectedLabels []string
    }{
        {
            name:   "successful getSlot",
            method: "getSlot",
            response: func(result interface{}) {
                *(result.(*json.RawMessage)) = json.RawMessage(`100`)
            },
            expectError: false,
            expectedLabels: []string{"test-node:8899", "getSlot", "block"},
        },
        {
            name:   "error getSlot",
            method: "getSlot",
            response: func(result interface{}) {
                return
            },
            expectError: true,
            expectedLabels: []string{"test-node:8899", "getSlot", "block", "request_failed"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient, metrics, collector := setupTest()

            mockClient.On("Call",
                mock.Anything,
                tt.method,
                mock.Anything,
                mock.Anything,
            ).Return(tt.response, nil)

            err := collector.measureRPCLatency(context.Background(), tt.method)

            if tt.expectError {
                assert.Error(t, err)
                assert.Equal(t, float64(1), metrics.RPCErrors.WithLabelValues(tt.expectedLabels...).Get())
            } else {
                assert.NoError(t, err)
                assert.Equal(t, float64(1), metrics.RPCRequests.WithLabelValues(tt.expectedLabels[:3]...).Get())
            }

            // Verify latency was recorded
            metric := metrics.RPCLatency.WithLabelValues(tt.expectedLabels[:3]...)
            assert.NotNil(t, metric)

            mockClient.AssertExpectations(t)
        })
    }
}

func TestCollector_MeasureRequestSizes(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    // Setup expectations for different methods
    for method := range methodCategories {
        mockClient.On("Call",
            mock.Anything,
            method,
            mock.Anything,
            mock.Anything,
        ).Return(func(result interface{}) {
            *(result.(*json.RawMessage)) = json.RawMessage(`{"test":"response"}`)
        }, nil)
    }

    err := collector.measureRequestSizes(context.Background())
    assert.NoError(t, err)

    // Verify that request sizes were recorded
    for method, category := range methodCategories {
        labels := []string{"test-node:8899", method, category}
        metric := metrics.RPCRequestSize.WithLabelValues(labels...)
        assert.NotNil(t, metric)
    }

    mockClient.AssertExpectations(t)
}

func TestCollector_TrackInFlightRequests(t *testing.T) {
    mockClient, metrics, collector := setupTest()

    mockClient.On("Call",
        mock.Anything,
        "getResourceConsumption",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        stats := result.(*struct {
            RPCRequests struct {
                Current int `json:"current"`
                Max     int `json:"max"`
            } `json:"rpcRequests"`
        })
        stats.RPCRequests.Current = 42
        stats.RPCRequests.Max = 100
    }, nil)

    err := collector.trackInFlightRequests(context.Background())
    assert.NoError(t, err)

    // Verify metrics
    assert.Equal(t, float64(42), metrics.RPCInFlight.WithLabelValues("test-node:8899").Get())
    mockClient.AssertExpectations(t)
}

func TestCollector_Collect(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup expectations for all methods
    for _, method := range monitoredMethods {
        mockClient.On("Call",
            mock.Anything,
            method,
            mock.Anything,
            mock.Anything,
        ).Return(func(result interface{}) {
            *(result.(*json.RawMessage)) = json.RawMessage(`{"test":"response"}`)
        }, nil)
    }

    // Setup resource consumption expectation
    mockClient.On("Call",
        mock.Anything,
        "getResourceConsumption",
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        stats := result.(*struct {
            RPCRequests struct {
                Current int `json:"current"`
                Max     int `json:"max"`
            } `json:"rpcRequests"`
        })
        stats.RPCRequests.Current = 42
    }, nil)

    err := collector.Collect(context.Background())
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}

func TestCollector_WithCancelledContext(t *testing.T) {
    _, _, collector := setupTest()

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

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

func TestDefaultParams(t *testing.T) {
    tests := []struct {
        method string
        want   bool // true if should return non-nil params
    }{
        {"getAccountInfo", true},
        {"getBlock", true},
        {"getTransaction", true},
        {"getProgramAccounts", true},
        {"getHealth", false},
        {"getVersion", false},
        {"nonexistentMethod", false},
    }

    for _, tt := range tests {
        t.Run(tt.method, func(t *testing.T) {
            params := getDefaultParams(tt.method)
            if tt.want {
                assert.NotNil(t, params)
            } else {
                assert.Nil(t, params)
            }
        })
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
    ).Return(func(result interface{}) {
        if rm, ok := result.(*json.RawMessage); ok {
            *rm = json.RawMessage(`{"test":"response"}`)
        }
    }, nil)

    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        collector.Collect(ctx)
    }
}

func TestMethodCategories(t *testing.T) {
    // Verify all monitored methods have categories
    for _, method := range monitoredMethods {
        category, exists := methodCategories[method]
        assert.True(t, exists, "Method %s should have a category", method)
        assert.NotEmpty(t, category, "Category for method %s should not be empty", method)
    }
}

func TestConcurrentCollection(t *testing.T) {
    mockClient, _, collector := setupTest()

    // Setup expectations
    mockClient.On("Call",
        mock.Anything,
        mock.Anything,
        mock.Anything,
        mock.Anything,
    ).Return(func(result interface{}) {
        if rm, ok := result.(*json.RawMessage); ok {
            *rm = json.RawMessage(`{"test":"response"}`)
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
    mockClient.AssertExpectations(t)
}
