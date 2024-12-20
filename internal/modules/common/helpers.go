// in internal/modules/common/helpers.go

package common

import (
    "context"
    "time"
)

// RetryWithTimeout attempts to execute a function with retries
func RetryWithTimeout(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
    var lastErr error
    for i := 0; i < attempts; i++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delay):
            }
        }
    }
    return lastErr
}

// SafeFloat64 safely converts interface{} to float64
func SafeFloat64(v interface{}) float64 {
    switch val := v.(type) {
    case float64:
        return val
    case float32:
        return float64(val)
    case int:
        return float64(val)
    case int64:
        return float64(val)
    case uint64:
        return float64(val)
    default:
        return 0
    }
}
