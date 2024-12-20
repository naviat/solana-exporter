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
    endpoint   string
    httpClient *http.Client
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
    Error   *struct {
        Code    int    `json:"code"`
        Message string `json:"message"`
    } `json:"error,omitempty"`
}

func NewClient(endpoint string, timeout time.Duration) *Client {
    return &Client{
        endpoint: endpoint,
        httpClient: &http.Client{
            Timeout: timeout,
        },
    }
}

func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
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

    var rpcResp RPCResponse
    if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
        return fmt.Errorf("decode response: %w", err)
    }

    if rpcResp.Error != nil {
        return fmt.Errorf("rpc error: %s", rpcResp.Error.Message)
    }

    if err := json.Unmarshal(rpcResp.Result, result); err != nil {
        return fmt.Errorf("unmarshal result: %w", err)
    }

    return nil
}