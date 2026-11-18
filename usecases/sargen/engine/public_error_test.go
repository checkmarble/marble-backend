package engine

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestPublicError(t *testing.T) {
	assert.Equal(t, "erreur technique du fournisseur LLM",
		PublicError(errors.Mark(errors.New("429 secret-token host=api.x"), ErrLLMCall)))
	assert.Equal(t, "délai dépassé", PublicError(errors.Wrap(context.DeadlineExceeded, "pq: x")))
	assert.Equal(t, "prompt manquant : a/b.md", PublicError(errors.Mark(errors.New("prompt manquant : a/b.md"), ErrPromptMissing)))
	assert.Equal(t, ErrRateLimitedTooLong.Error(), PublicError(models.LLMRateLimitedError))
	assert.Equal(t, ErrJobNotRunning.Error(), PublicError(errors.Wrap(ErrJobNotRunning, "apply")))
	assert.Equal(t, "erreur technique pendant la génération", PublicError(errors.New("pq: connection refused 10.0.0.1")))
	assert.Equal(t, "génération impossible : modèle non configuré",
		PublicError(unavailable(errors.New("modèle non configuré"))))
}
