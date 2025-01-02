package solana

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "log"
    "net"           // for SplitHostPort
    "strconv"       // for Atoi
    "sync"
    "time"
    "strings"

    "github.com/gorilla/websocket"
)

type Client struct {
    endpoint         string
    httpClient      *http.Client
    labels          map[string]string
    retryConfig     RetryConfig
    inflightRequests map[int64]struct{}  // Track in-flight requests
    nextRequestID   int64                // For generating unique request IDs
    mu              sync.RWMutex         // Protect concurrent access

    // WebSocket fields
    wsEndpoint      string
    wsConn         *websocket.Conn
    wsSubscriptions map[string]map[string]struct{} // type -> subscription_ids
    wsMu           sync.RWMutex
    wsHandlers     map[string]func([]byte)
    wsConnected    bool

    httpPort        string    // Track HTTP port
    wsPort          int    // Track WS port

    activeSubscriptions sync.Map
}

type RetryConfig struct {
    MaxRetries   int
    RetryBackoff time.Duration
}

type RPCRequest struct {
    Jsonrpc string        `json:"jsonrpc"`
    ID      int64         `json:"id"`  // Changed to int64 to match nextRequestID
    Method  string        `json:"method"`
    Params  []interface{} `json:"params,omitempty"`
}

type RPCResponse struct {
    Jsonrpc string          `json:"jsonrpc"`
    ID      int64           `json:"id"`  // Changed to int64 to match request
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

// NewClient creates a new client with separate HTTP and WebSocket endpoints
func NewClient(endpoint string, wsPort int, timeout time.Duration, labels map[string]string) *Client {
    // Parse the endpoint correctly
    httpURL, err := url.Parse(endpoint)
    if err != nil {
        return nil
    }

    // Extract the host part correctly, handling cases like "localhost:8799" or "http://localhost:8799"
    var host string
    if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
        host = httpURL.Hostname()
    } else {
        // If no scheme, split by colon to get host
        host = strings.Split(endpoint, ":")[0]
    }

    // Create WebSocket endpoint
    wsEndpoint := fmt.Sprintf("ws://%s:%d", host, wsPort)

    client := &Client{
        endpoint:         endpoint,
        wsEndpoint:      wsEndpoint,
        httpClient:      &http.Client{Timeout: timeout},
        inflightRequests: make(map[int64]struct{}),
        wsSubscriptions:  make(map[string]map[string]struct{}),
        wsHandlers:      make(map[string]func([]byte)),
        labels:          labels,
        wsPort:         wsPort,
    }

    // Start WebSocket connection
    go func() {
        ctx := context.Background()
        if err := client.ConnectWebSocket(ctx); err != nil {
            log.Printf("Failed to connect WebSocket: %v", err)
        }
    }()

    return client
}

func (c *Client) GetLabels() map[string]string {
    return c.labels
}

func (c *Client) SetRetryConfig(config RetryConfig) {
    c.retryConfig = config
}

// Add method to get WS endpoint for labels
func (c *Client) GetWSAddress() string {
    host := strings.Split(c.labels["node_address"], ":")[0]
    return fmt.Sprintf("%s:%d", host, c.wsPort)
}

func (c *Client) GetWSPort() int {
    // Parse WebSocket URL to get port
    wsURL, err := url.Parse(c.wsEndpoint)
    if err != nil {
        return 8800 // Default WS port as fallback
    }
    
    // Extract port from host
    _, portStr, err := net.SplitHostPort(wsURL.Host)
    if err != nil {
        return 8800
    }
    
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return 8800
    }
    
    return port
}

// GetInflightRequests returns the number of in-flight requests
func (c *Client) GetInflightRequests() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.inflightRequests)
}

func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
    var lastErr error
    backoff := time.Second

    for retry := 0; retry <= c.retryConfig.MaxRetries; retry++ {
        if retry > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(backoff):
                backoff *= 2 // exponential backoff
            }
        }

        // Make the RPC call
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
            lastErr = fmt.Errorf("do request: %w", err)
            continue
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
            continue
        }

        var rpcResp RPCResponse
        if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
            lastErr = fmt.Errorf("decode response: %w", err)
            continue
        }

        if rpcResp.Error != nil {
            log.Printf("RPC error response: %+v", rpcResp.Error)
            // Don't retry certain RPC errors
            switch rpcResp.Error.Code {
            case -32601, // Method not found
                 -32602: // Invalid params
                return rpcResp.Error
            default:
                lastErr = rpcResp.Error
                continue
            }
        }

        if result != nil {
            if err := json.Unmarshal(rpcResp.Result, result); err != nil {
                lastErr = fmt.Errorf("unmarshal result: %w", err)
                continue
            }
        }

        return nil // Success
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doCall(ctx context.Context, method string, params []interface{}, result interface{}) error {
    // Generate unique request ID and track it
    c.mu.Lock()
    requestID := c.nextRequestID
    c.nextRequestID++
    c.inflightRequests[requestID] = struct{}{}
    c.mu.Unlock()

    // Ensure we remove the request from tracking when done
    defer func() {
        c.mu.Lock()
        delete(c.inflightRequests, requestID)
        c.mu.Unlock()
    }()

    request := RPCRequest{
        Jsonrpc: "2.0",
        ID:      requestID,
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

// WebSocket methods
func (c *Client) ConnectWebSocket(ctx context.Context) error {
    c.wsMu.Lock()
    defer c.wsMu.Unlock()

    if c.wsConnected {
        return nil
    }

    dialer := websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
    }

    conn, _, err := dialer.DialContext(ctx, c.wsEndpoint, nil)
    if err != nil {
        return fmt.Errorf("websocket connection failed: %w", err)
    }

    c.wsConn = conn
    c.wsConnected = true

    // Start message reader
    go c.wsReadMessages()

    return nil
}

func (c *Client) Subscribe(ctx context.Context, subType string, params []interface{}, handler func([]byte)) error {
    c.wsMu.Lock()
    defer c.wsMu.Unlock()

    // Connect if not connected
    if !c.wsConnected {
        if err := c.ConnectWebSocket(ctx); err != nil {
            return fmt.Errorf("failed to connect websocket: %w", err)
        }
    }

    // Create subscription request
    request := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      c.nextRequestID,
        "method":  subType + "Subscribe",
        "params":  params,
    }

    // Send subscription request
    if err := c.wsConn.WriteJSON(request); err != nil {
        return fmt.Errorf("subscription request failed: %w", err)
    }

    // Initialize subscription type map if needed
    if _, ok := c.wsSubscriptions[subType]; !ok {
        c.wsSubscriptions[subType] = make(map[string]struct{})
    }

    // Store subscription with ID
    subID := fmt.Sprintf("%d", c.nextRequestID)
    c.wsSubscriptions[subType][subID] = struct{}{}
    c.wsHandlers[subID] = handler
    c.nextRequestID++

    return nil
}

// Add unsubscribe method
func (c *Client) Unsubscribe(ctx context.Context, subType string, subID string) error {
    c.wsMu.Lock()
    defer c.wsMu.Unlock()

    if !c.wsConnected {
        return fmt.Errorf("websocket not connected")
    }

    // Remove subscription tracking
    if subs, ok := c.wsSubscriptions[subType]; ok {
        delete(subs, subID)
    }
    delete(c.wsHandlers, subID)

    // Send unsubscribe request
    request := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      c.nextRequestID,
        "method":  subType + "Unsubscribe",
        "params":  []interface{}{subID},
    }

    return c.wsConn.WriteJSON(request)
}

func (c *Client) wsReadMessages() {
    for {
        _, message, err := c.wsConn.ReadMessage()
        if err != nil {
            log.Printf("WebSocket read error: %v", err)
            c.wsMu.Lock()
            c.wsConnected = false
            c.wsMu.Unlock()
            return
        }

        // Parse message
        var msg struct {
            ID      interface{}     `json:"id"`           // Can be string or number
            Method  string          `json:"method"`
            Params  json.RawMessage `json:"params"`
            Result  interface{}     `json:"result"`       // For subscription responses
            Error   *RPCError       `json:"error"`
        }

        if err := json.Unmarshal(message, &msg); err != nil {
            log.Printf("Failed to unmarshal WS message: %v", err)
            continue
        }
        // Log the received message for debugging
        log.Printf("Received WS message: Method=%s, ID=%v, Result=%v", msg.Method, msg.ID, msg.Result)
        // Track subscriptions
        if msg.Result != nil {
            subID := fmt.Sprintf("%v", msg.Result)
            log.Printf("Got subscription ID: %s for method: %s", subID, msg.Method)
            
            // Handle subscribe methods
            if strings.Contains(msg.Method, "Subscribe") {
                subType := strings.TrimSuffix(msg.Method, "Subscribe")
                c.wsMu.Lock()
                if c.wsSubscriptions == nil {
                    c.wsSubscriptions = make(map[string]map[string]struct{})
                }
                if c.wsSubscriptions[subType] == nil {
                    c.wsSubscriptions[subType] = make(map[string]struct{})
                }
                c.wsSubscriptions[subType][subID] = struct{}{}
                c.wsMu.Unlock()
                log.Printf("Added subscription: type=%s, id=%s", subType, subID)
            }
        }
        // // Handle unsubscribe
        // if strings.HasSuffix(msg.Method, "Unsubscribe") {
        //     // Need to unmarshal Params into a separate structure
        //     var params []string
        //     if err := json.Unmarshal(msg.Params, &params); err == nil {
        //         if len(params) > 0 {
        //             c.activeSubscriptions.Delete(params[0])
        //         }
        //     }
        // }

        // // Handle subscription notifications
        // if msg.Method != "" && msg.Params != nil {
        //     if handler, ok := c.wsHandlers[fmt.Sprintf("%v", msg.ID)]; ok {
        //         handler(msg.Params)
        //     }
        // }
    }
}

// Metrics methods
func (c *Client) GetWSConnectionCount() int {
    c.wsMu.RLock()
    defer c.wsMu.RUnlock()
    if c.wsConnected {
        return 1
    }
    return 0
}

// func (c *Client) GetWSSubscriptionCount(subType string) int {
//     count := 0
//     c.activeSubscriptions.Range(func(_, value interface{}) bool {
//         if value.(string) == subType {
//             count++
//         }
//         return true
//     })
//     return count
// }

// Update GetWSSubscriptionCount to also log
func (c *Client) GetWSSubscriptionCount(subType string) int {
    c.wsMu.RLock()
    defer c.wsMu.RUnlock()
    count := len(c.wsSubscriptions[subType])
    log.Printf("Getting subscription count for %s: %d", subType, count)
    return count
}

// // Add helper method to manually store subscription (optional)
// func (c *Client) StoreSubscription(subType string, subID string) {
//     c.activeSubscriptions.Store(subID, subType)
// }