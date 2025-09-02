package utils

import (
	"testing"
	"time"
)

var (
	globalCacheDuration = 30 * time.Second
)

func GlobalCacheDuration() time.Duration {
	if testing.Testing() {
		return time.Microsecond
	}

	return globalCacheDuration
}
