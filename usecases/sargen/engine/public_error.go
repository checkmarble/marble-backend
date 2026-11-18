package engine

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
)

// ErrLLMCall marks a failure of a call to the model provider.
var ErrLLMCall = errors.New("appel du fournisseur LLM en échec")

// PublicError is the message of err that may be stored on a job or task and
// shown to the analyst: French, without provider or internal detail. The raw
// text goes to the logs (and the task trace) only.
func PublicError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrPromptMissing):
		return err.Error() // « prompt manquant : <chemin> »
	case errors.Is(err, ErrLLMUnavailable):
		// Configuration errors are our own French messages (no provider detail).
		return "génération impossible : " + errors.UnwrapAll(err).Error()
	case errors.Is(err, ErrJobNotRunning):
		return ErrJobNotRunning.Error()
	case errors.Is(err, ErrTaskLimitReached):
		return ErrTaskLimitReached.Error()
	case errors.Is(err, ErrRateLimitedTooLong), errors.Is(err, models.LLMRateLimitedError):
		return ErrRateLimitedTooLong.Error()
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "délai dépassé"
	case errors.Is(err, ErrLLMCall):
		return "erreur technique du fournisseur LLM"
	default:
		return "erreur technique pendant la génération"
	}
}
