//go:build windows

package main

import (
	"time"

	"golang.org/x/sys/windows"
)

// cpuTime is the process's user + kernel CPU time.
func cpuTime() time.Duration {
	var c, e, k, u windows.Filetime
	if windows.GetProcessTimes(windows.CurrentProcess(), &c, &e, &k, &u) != nil {
		return 0
	}
	ft := func(f windows.Filetime) time.Duration {
		return time.Duration(int64(f.HighDateTime)<<32|int64(f.LowDateTime)) * 100
	}
	return ft(k) + ft(u)
}
