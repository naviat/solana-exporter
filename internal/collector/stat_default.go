//go:build !linux && !darwin
package collector

import (
    "os"
    "time"
)

func getProcessStartTime(info os.FileInfo) int64 {
    return info.ModTime().Unix()
}
