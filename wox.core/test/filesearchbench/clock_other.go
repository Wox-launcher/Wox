//go:build !windows

package filesearchbench

import "time"

var queryEpoch = time.Now()

func queryClock() int64 { return time.Since(queryEpoch).Nanoseconds() }
