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

// WSConnection represents a WebSocket connection and its statistics
type WSConnection struct {
    conn          *websocket.Conn
    subscriptions map[string]int64
    messageCount  atomic.Int64
    errorCount   atomic.Int64
    lastMessage  time.Time
    mu           sync.RWMutex
}

// Client represents a Solana RPC and WebSocket client
type Client struct {
    // HTTP RPC settings
    endpoint         string
    httpClient      *http.Client
    timeout         time.Duration
    maxRetries      int
    requestIDCounter atomic.Int64
    rateLimiter     *time.Ticker

    // WebSocket settings
    wsEndpoint      string
    wsConn         *WSConnection
    wsEnabled      bool
    maxConnections  int
    reconnectDelay  time.Duration
    
    // Metrics tracking
    inflightRequests sync.Map
    lastResponseTime atomic.Int64
}

// NewClient creates a new Solana client with both RPC and optional WebSocket support
func NewClient(endpoint string, wsEndpoint string, timeout time.Duration, maxRetries int, maxRPS int) *Client {
    return &Client{
        endpoint:        endpoint,
        wsEndpoint:     wsEndpoint,
        httpClient:     &http.Client{Timeout: timeout},
        timeout:        timeout,
        maxRetries:     maxRetries,
        wsEnabled:      wsEndpoint != "",
        maxConnections: 5,
        reconnectDelay: 5 * time.Second,
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

// ConnectWebSocket establishes a WebSocket connection
func (c *Client) ConnectWebSocket(ctx context.Context) error {
    if !c.wsEnabled {
        return fmt.Errorf("websocket not enabled")
    }

    dialer := websocket.Dialer{
        HandshakeTimeout: c.timeout,
    }

    conn, _, err := dialer.DialContext(ctx, c.wsEndpoint, nil)
    if err != nil {
        return fmt.Errorf("connect websocket: %w", err)
    }

    c.wsConn = &WSConnection{
        conn:          conn,
        subscriptions: make(map[string]int64),
        lastMessage:   time.Now(),
    }

    // Start message handler
    go c.handleWebSocketMessages()

    return nil
}

// Subscribe creates a WebSocket subscription
func (c *Client) Subscribe(ctx context.Context, method string, params []interface{}) error {
    if c.wsConn == nil {
        return fmt.Errorf("websocket not connected")
    }

    request := RPCRequest{
        Jsonrpc: "2.0",
        ID:      c.requestIDCounter.Add(1),
        Method:  method,
        Params:  params,
    }

    c.wsConn.mu.Lock()
    defer c.wsConn.mu.Unlock()

    return c.wsConn.conn.WriteJSON(request)
}

// handleWebSocketMessages processes incoming WebSocket messages
func (c *Client) handleWebSocketMessages() {
    for {
        if c.wsConn == nil {
            return
        }

        _, message, err := c.wsConn.conn.ReadMessage()
        if err != nil {
            c.wsConn.errorCount.Add(1)
            // Attempt reconnection
            if websocket.IsUnexpectedCloseError(err) {
                go c.reconnectWebSocket()
            }
            return
        }

        c.wsConn.messageCount.Add(1)
        c.wsConn.mu.Lock()
        c.wsConn.lastMessage = time.Now()
        c.wsConn.mu.Unlock()

        // Process message (you can add message handling logic here)
        var response RPCResponse
        if err := json.Unmarshal(message, &response); err != nil {
            c.wsConn.errorCount.Add(1)
            continue
        }
    }
}

// GetWSStats returns WebSocket connection statistics
func (c *Client) GetWSStats() map[string]interface{} {
    if c.wsConn == nil {
        return map[string]interface{}{
            "connected": false,
        }
    }

    c.wsConn.mu.RLock()
    defer c.wsConn.mu.RUnlock()

    return map[string]interface{}{
        "connected":         true,
        "message_count":     c.wsConn.messageCount.Load(),
        "error_count":       c.wsConn.errorCount.Load(),
        "subscription_count": len(c.wsConn.subscriptions),
        "last_message":      c.wsConn.lastMessage,
    }
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

func (c *Client) reconnectWebSocket() {
    time.Sleep(c.reconnectDelay)
    
    ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
    defer cancel()

    if err := c.ConnectWebSocket(ctx); err != nil {
        // Could implement exponential backoff here
        return
    }

    // Resubscribe to previous subscriptions
    c.wsConn.mu.RLock()
    subscriptions := make(map[string]int64, len(c.wsConn.subscriptions))
    for method, id := range c.wsConn.subscriptions {
        subscriptions[method] = id
    }
    c.wsConn.mu.RUnlock()

    for method := range subscriptions {
        if err := c.Subscribe(ctx, method, nil); err != nil {
            c.wsConn.errorCount.Add(1)
        }
    }
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

// GetWSConnectionCount returns the number of active WebSocket connections
func (c *Client) GetWSConnectionCount() int {
    if c.wsConn != nil && c.wsConn.conn != nil {
        return 1
    }
    return 0
}

// GetWSSubscriptionCount returns the number of active subscriptions
func (c *Client) GetWSSubscriptionCount() int {
    if c.wsConn == nil {
        return 0
    }
    c.wsConn.mu.RLock()
    defer c.wsConn.mu.RUnlock()
    return len(c.wsConn.subscriptions)
}

// Close closes the client and all connections
func (c *Client) Close() error {
    c.rateLimiter.Stop()
    
    if c.wsConn != nil && c.wsConn.conn != nil {
        return c.wsConn.conn.Close()
    }
    
    return nil
}
