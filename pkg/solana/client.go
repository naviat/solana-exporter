package solana

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    endpoint    string
    httpClient  *http.Client
    labels      map[string]string
    retryConfig RetryConfig
}

type RetryConfig struct {
    MaxRetries   int
    RetryBackoff time.Duration
}

type RPCRequest struct {
    Jsonrpc string        `json:"jsonrpc"`
    ID      int           `json:"id"`
    Method  string        `json:"method"`
    Params  []interface{} `json:"params,omitempty"`
}

type RPCResponse struct {
    Jsonrpc string          `json:"jsonrpc"`
    ID      int             `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

func (e *RPCError) Error() string {
    return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

func NewClient(endpoint string, timeout time.Duration, labels map[string]string) *Client {
    if labels == nil {
        labels = make(map[string]string)
    }
    
    return &Client{
        endpoint: endpoint,
        httpClient: &http.Client{
            Timeout: timeout,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 100,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        labels: labels,
        retryConfig: RetryConfig{
            MaxRetries:   3,
            RetryBackoff: time.Second,
        },
    }
}

func (c *Client) GetLabels() map[string]string {
    return c.labels
}

func (c *Client) SetRetryConfig(config RetryConfig) {
    c.retryConfig = config
}

func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    var lastErr error
    for retry := 0; retry <= c.retryConfig.MaxRetries; retry++ {
        if err := c.doCall(ctx, method, params, result); err != nil {
            lastErr = err
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(c.retryConfig.RetryBackoff):
                continue
            }
        } else {
            return nil
        }
    }
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doCall(ctx context.Context, method string, params []interface{}, result interface{}) error {
    request := RPCRequest{
        Jsonrpc: "2.0",
        ID:      1,
        Method:  method,
        Params:  params,
    }

    jsonData, err := json.Marshal(request)
    if err != nil {
        return fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var rpcResp RPCResponse
    if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
        return fmt.Errorf("decode response: %w", err)
    }

    if rpcResp.Error != nil {
        return rpcResp.Error
    }

    if result == nil {
        return nil
    }

    if err := json.Unmarshal(rpcResp.Result, result); err != nil {
        return fmt.Errorf("unmarshal result: %w", err)
    }

    return nil
}
