//go:build darwin
package collector

import (
    "os"
    "syscall"
)

func getProcessStartTime(info os.FileInfo) int64 {
    if stat, ok := info.Sys().(*syscall.Stat_t); ok {
        return stat.Birthtimespec.Sec
    }
    return 0
}
