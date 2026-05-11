package ai_agent

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/llmberjack"
	"github.com/cockroachdb/errors"
)

// IsLLMRateLimitError reports whether err corresponds to an upstream LLM
// provider rate-limit / resource-exhausted response. Matching is done on the
// error string because the provider SDKs do not expose a stable typed shape.
func IsLLMRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Error 429") ||
		strings.Contains(errStr, "Error 504") ||
		strings.Contains(errStr, "Resource exhausted") ||
		strings.Contains(errStr, "RESOURCE_EXHAUSTED") ||
		strings.Contains(errStr, "DEADLINE_EXCEEDED")
}

// WrapIfLLMRateLimit tags err with the models.LLMRateLimitedError marker when
// IsLLMRateLimitError matches, otherwise returns err unchanged. The original
// error chain (wraps, stack frames, provider-specific types) is preserved so
// that errors.Is(result, models.LLMRateLimitedError) reports true while
// upstream details remain available for debugging.
func WrapIfLLMRateLimit(err error) error {
	if err == nil || !IsLLMRateLimitError(err) {
		return err
	}
	return errors.Mark(err, models.LLMRateLimitedError)
}

// DoLLMRequest is the single sanctioned entrypoint for executing an
// llmberjack request. It tags upstream rate-limit / resource-exhausted
// responses with models.LLMRateLimitedError so callers and presentError
// can react with typed checks instead of string matching. New LLM calls
// must go through this helper rather than calling req.Do directly.
func DoLLMRequest[T any](
	ctx context.Context,
	client *llmberjack.Llmberjack,
	req llmberjack.Request[T],
) (*llmberjack.Response[T], error) {
	resp, err := req.Do(ctx, client)
	return resp, WrapIfLLMRateLimit(err)
}
