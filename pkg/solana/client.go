package solana

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"

    "github.com/gorilla/websocket"
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

type Client struct {
    endpoint         string
    wsEndpoint      string
    httpClient      *http.Client
    wsConn         *websocket.Conn
    
    // Request tracking
    inflightRequests map[int64]struct{}
    requestIDCounter int64
    requestMu        sync.RWMutex
    
    // WebSocket management
    wsSubscriptions  map[string]map[string]struct{}
    wsMessageCounts  map[string]uint64
    wsMu            sync.RWMutex
    wsConnected     bool
    
    // Configuration
    timeout         time.Duration
    maxRetries      int
    wsPort          int
}

// parseHost extracts the host from an endpoint URL
func parseHost(endpoint string) string {
    u, err := url.Parse(endpoint)
    if err != nil {
        return strings.Split(endpoint, "://")[1]
    }
    return strings.Split(u.Host, ":")[0]
}

// NewClient creates a new Solana RPC client
func NewClient(endpoint string, wsPort int, timeout time.Duration, maxRetries int) *Client {
    return &Client{
        endpoint:         endpoint,
        wsEndpoint:      fmt.Sprintf("ws://%s:%d", parseHost(endpoint), wsPort),
        httpClient:      &http.Client{Timeout: timeout},
        inflightRequests: make(map[int64]struct{}),
        wsSubscriptions:  make(map[string]map[string]struct{}),
        wsMessageCounts:  make(map[string]uint64), // Initialize message counts
        timeout:         timeout,
        maxRetries:      maxRetries,
        wsPort:          wsPort,
    }
}

// Call makes an HTTP RPC request with retries and timeout
func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    var lastErr error
    backoff := time.Second

    for retry := 0; retry <= c.maxRetries; retry++ {
        if retry > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff):
                backoff *= 2 // exponential backoff
            }
        }

        // Track in-flight request
        requestID := c.trackRequest()
        defer c.untrackRequest(requestID)

        // Prepare request
        request := RPCRequest{
            Jsonrpc: "2.0",
            ID:      requestID,
            Method:  method,
            Params:  params,
        }

        response, err := c.doRequest(ctx, request)
        if err != nil {
            lastErr = err
            continue
        }

        // Handle response
        if response.Error != nil {
            lastErr = response.Error
            if !isRetryableError(response.Error.Code) {
                return lastErr
            }
            continue
        }

        // Unmarshal result if provided
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

// WebSocket connection management
func (c *Client) ConnectWebSocket(ctx context.Context) error {
    c.wsMu.Lock()
    defer c.wsMu.Unlock()

    if c.wsConn != nil {
        return nil
    }

    dialer := websocket.Dialer{
        HandshakeTimeout: c.timeout,
    }

    conn, _, err := dialer.DialContext(ctx, c.wsEndpoint, nil)
    if err != nil {
        return fmt.Errorf("websocket connection failed: %w", err)
    }

    c.wsConn = conn
    go c.handleWebSocketMessages()

    return nil
}

// Helper methods for request tracking
func (c *Client) trackRequest() int64 {
    c.requestMu.Lock()
    defer c.requestMu.Unlock()
    
    c.requestIDCounter++
    id := c.requestIDCounter
    c.inflightRequests[id] = struct{}{}
    return id
}

func (c *Client) untrackRequest(id int64) {
    c.requestMu.Lock()
    defer c.requestMu.Unlock()
    delete(c.inflightRequests, id)
}

// Metric collection helpers
func (c *Client) GetInflightRequests() int {
    c.requestMu.RLock()
    defer c.requestMu.RUnlock()
    return len(c.inflightRequests)
}

func (c *Client) GetWSConnectionCount() int {
    c.wsMu.RLock()
    defer c.wsMu.RUnlock()
    if c.wsConn != nil {
        return 1
    }
    return 0
}

func (c *Client) GetWSSubscriptionCount(subType string) int {
    c.wsMu.RLock()
    defer c.wsMu.RUnlock()
    return len(c.wsSubscriptions[subType])
}

// Private helper methods
func (c *Client) doRequest(ctx context.Context, request RPCRequest) (*RPCResponse, error) {
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
    retryableCodes := map[int]bool{
        -32007: true, // Node is behind
        -32008: true, // Node is busy
        -32009: true, // Transaction simulation failed
    }
    return retryableCodes[code]
}

func (c *Client) GetWSMessageCount(direction string) uint64 {
    c.wsMu.RLock()
    defer c.wsMu.RUnlock()
    return c.wsMessageCounts[direction]
}

func (c *Client) incrementWSMessageCount(direction string) {
    c.wsMu.Lock()
    defer c.wsMu.Unlock()
    c.wsMessageCounts[direction]++
}

// handleWebSocketMessages processes incoming WebSocket messages
func (c *Client) handleWebSocketMessages() {
    for {
        if !c.wsConnected {
            return
        }

        _, message, err := c.wsConn.ReadMessage()
        if err != nil {
            c.wsMu.Lock()
            c.wsConnected = false
            c.wsMu.Unlock()
            return
        }

        // Increment inbound message count
        c.incrementWSMessageCount("inbound")

        // Process the message
        var msg struct {
            Method  string          `json:"method"`
            Params  json.RawMessage `json:"params"`
            ID      interface{}     `json:"id,omitempty"`
            Result  json.RawMessage `json:"result,omitempty"`
            Error   *RPCError       `json:"error,omitempty"`
        }

        if err := json.Unmarshal(message, &msg); err != nil {
            continue
        }

        // Handle subscription responses and increment outbound count for responses
        if msg.Result != nil && msg.ID != nil {
            c.incrementWSMessageCount("outbound")
            var subID uint64
            if err := json.Unmarshal(msg.Result, &subID); err == nil {
                c.wsMu.Lock()
                if c.wsSubscriptions[msg.Method] == nil {
                    c.wsSubscriptions[msg.Method] = make(map[string]struct{})
                }
                c.wsSubscriptions[msg.Method][fmt.Sprintf("%d", subID)] = struct{}{}
                c.wsMu.Unlock()
            }
        }
    }
}
