package utils

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

// IsTransientNetworkError reports whether err is a transient network-level
// failure we don't want to capture in Sentry: socket timeouts and
// remote-closed connections (broken pipe / connection reset). The returned
// kind is a stable label suitable for metric/log fields.
func IsTransientNetworkError(err error) (kind string, ok bool) {
	if err == nil {
		return "", false
	}
	switch {
	case errors.Is(err, syscall.EPIPE):
		return "broken_pipe", true
	case errors.Is(err, syscall.ECONNRESET):
		return "conn_reset", true
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected_eof", true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout", true
	}
	return "", false
}

// MaybeSuppressTransient handles transient network errors uniformly: if err
// matches, it is logged at warn level and counted via Prometheus, and the
// function returns true so callers can skip Sentry capture.
func MaybeSuppressTransient(ctx context.Context, err error) bool {
	kind, ok := IsTransientNetworkError(err)
	if !ok {
		return false
	}
	LoggerFromContext(ctx).WarnContext(ctx, "transient network error",
		"kind", kind,
		"error", err.Error(),
	)
	MetricTransientNetworkErrors.WithLabelValues(kind).Inc()
	return true
}
