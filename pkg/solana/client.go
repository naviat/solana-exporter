package solana

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "sync/atomic"
    "time"
)

// RPCRequest represents a JSON-RPC 2.0 request
type RPCRequest struct {
    Jsonrpc string        `json:"jsonrpc"`
    ID      int64         `json:"id"`
    Method  string        `json:"method"`
    Params  []interface{} `json:"params,omitempty"`
}

// RPCResponse represents a JSON-RPC 2.0 response
type RPCResponse struct {
    Jsonrpc string          `json:"jsonrpc"`
    ID      int64           `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error
type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func (e *RPCError) Error() string {
    return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Client represents a Solana RPC client
type Client struct {
    endpoint         string
    httpClient      *http.Client
    timeout         time.Duration
    maxRetries      int
    requestIDCounter atomic.Int64
    rateLimiter     *time.Ticker
    
    // Metrics tracking
    inflightRequests sync.Map
    lastResponseTime atomic.Int64
}

// NewClient creates a new Solana RPC client
func NewClient(endpoint string, timeout time.Duration, maxRetries int, maxRPS int) *Client {
    return &Client{
        endpoint:        endpoint,
        httpClient:     &http.Client{Timeout: timeout},
        timeout:        timeout,
        maxRetries:     maxRetries,
        rateLimiter:    time.NewTicker(time.Second / time.Duration(maxRPS)),
    }
}

// Call makes an HTTP RPC request with retries and timeout
func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    var lastErr error
    backoff := c.timeout / time.Duration(c.maxRetries)

    for retry := 0; retry <= c.maxRetries; retry++ {
        if retry > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff * time.Duration(retry)):
            }
        }

        // Wait for rate limiter
        select {
        case <-c.rateLimiter.C:
        case <-ctx.Done():
            return ctx.Err()
        }

        // Track request
        requestID := c.requestIDCounter.Add(1)
        c.inflightRequests.Store(requestID, time.Now())
        defer c.inflightRequests.Delete(requestID)

        // Make the request
        response, err := c.doRequest(ctx, method, params, requestID)
        if err != nil {
            lastErr = err
            continue
        }

        // Handle response
        if response.Error != nil {
            lastErr = response.Error
            // Don't retry if it's not a retryable error
            if !isRetryableError(response.Error.Code) {
                return lastErr
            }
            continue
        }

        // Unmarshal result
        if result != nil && response.Result != nil {
            if err := json.Unmarshal(response.Result, result); err != nil {
                lastErr = fmt.Errorf("unmarshal result: %w", err)
                continue
            }
        }

        return nil
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Private helper methods
func (c *Client) doRequest(ctx context.Context, method string, params []interface{}, id int64) (*RPCResponse, error) {
    request := RPCRequest{
        Jsonrpc: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    jsonData, err := json.Marshal(request)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    // Update last response time
    c.lastResponseTime.Store(time.Now().UnixNano())

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    var rpcResp RPCResponse
    if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &rpcResp, nil
}

func isRetryableError(code int) bool {
    return code == -32007 || // Node is behind
           code == -32008 || // Node is busy
           code == -32009    // Transaction simulation failed
}

// GetInflightRequests returns the number of in-flight requests
func (c *Client) GetInflightRequests() int {
    count := 0
    c.inflightRequests.Range(func(key, value interface{}) bool {
        count++
        return true
    })
    return count
}

// Close cleans up the client resources
func (c *Client) Close() error {
    c.rateLimiter.Stop()
    return nil
}
