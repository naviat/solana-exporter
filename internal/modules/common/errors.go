package common

import "fmt"

// HandleErrors collects errors from error channel and returns a combined error if any
func HandleErrors(errCh chan error) error {
    var errs []error
    for err := range errCh {
        if err != nil {
            errs = append(errs, err)
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("multiple collection errors: %v", errs)
    }
    return nil
}