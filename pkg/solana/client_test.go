package solana

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
    server := httptest.NewServer(handler)
    client := NewClient(server.URL, 5*time.Second, map[string]string{
        "test_label": "test_value",
    })
    return client, server
}

func TestClient_Call(t *testing.T) {
    tests := []struct {
        name           string
        method        string
        params        []interface{}
        response      string
        expectedError string
        expectedResult interface{}
    }{
        {
            name:    "successful call",
            method: "getHealth",
            response: `{"jsonrpc":"2.0","id":1,"result":"ok"}`,
            expectedResult: "ok",
        },
        {
            name:    "rpc error",
            method: "getHealth",
            response: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`,
            expectedError: "rpc error -32600: Invalid request",
        },
        {
            name:    "invalid json response",
            method: "getHealth",
            response: `invalid json`,
            expectedError: "decode response:",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
                // Verify request
                var req RPCRequest
                err := json.NewDecoder(r.Body).Decode(&req)
                require.NoError(t, err)
                assert.Equal(t, tt.method, req.Method)
                assert.Equal(t, tt.params, req.Params)

                // Send response
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(tt.response))
            })
            defer server.Close()

            var result interface{}
            err := client.Call(context.Background(), tt.method, tt.params, &result)

            if tt.expectedError != "" {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.expectedResult, result)
            }
        })
    }
}

func TestClient_RetryBehavior(t *testing.T) {
    attempts := 0
    client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
        attempts++
        if attempts <= 2 {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
    })
    defer server.Close()

    client.SetRetryConfig(RetryConfig{
        MaxRetries:   3,
        RetryBackoff: 10 * time.Millisecond,
    })

    var result string
    err := client.Call(context.Background(), "getHealth", nil, &result)
    assert.NoError(t, err)
    assert.Equal(t, "ok", result)
    assert.Equal(t, 3, attempts)
}

func TestClient_ContextCancellation(t *testing.T) {
    client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(100 * time.Millisecond)
        w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`))
    })
    defer server.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    var result string
    err := client.Call(ctx, "getHealth", nil, &result)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClient_Labels(t *testing.T) {
    labels := map[string]string{
        "node_type": "rpc",
        "org":       "test",
    }
    client := NewClient("http://localhost:8899", 5*time.Second, labels)
    
    // Check if labels are stored correctly
    assert.Equal(t, labels, client.GetLabels())
}

func TestClient_CommonMethods(t *testing.T) {
    client, server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
        var req RPCRequest
        json.NewDecoder(r.Body).Decode(&req)

        var response string
        switch req.Method {
        case "getHealth":
            response = `{"jsonrpc":"2.0","id":1,"result":"ok"}`
        case "getSlot":
            response = `{"jsonrpc":"2.0","id":1,"result":12345}`
        case "getVersion":
            response = `{"jsonrpc":"2.0","id":1,"result":{"solana-core":"1.14.10"}}`
        }

        w.Write([]byte(response))
    })
    defer server.Close()

    t.Run("GetHealth", func(t *testing.T) {
        health, err := client.GetHealth(context.Background())
        assert.NoError(t, err)
        assert.Equal(t, "ok", health)
    })

    t.Run("GetSlot", func(t *testing.T) {
        slot, err := client.GetSlot(context.Background())
        assert.NoError(t, err)
        assert.Equal(t, uint64(12345), slot)
    })

    t.Run("GetVersion", func(t *testing.T) {
        version, err := client.GetVersion(context.Background())
        assert.NoError(t, err)
        assert.Equal(t, "1.14.10", version["solana-core"])
    })
}
