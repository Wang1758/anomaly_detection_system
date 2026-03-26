package perf

import (
	"log"
	"sync/atomic"
)

var enabled atomic.Bool

func SetEnabled(v bool) {
	enabled.Store(v)
}

func Enabled() bool {
	return enabled.Load()
}

func Logf(format string, args ...interface{}) {
	if !enabled.Load() {
		return
	}
	log.Printf(format, args...)
}
