package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTracfinRevisionGuardPrefillAndValidate(t *testing.T) {
	org := uuid.New()
	repo := &draftTestRepo{configErr: models.NotFoundError}
	uc := draftTestUsecase(org, repo, models.CaseConfirmedRisk)
	ctx := draftTestContext(org)
	draft, err := uc.CreateDraft(ctx, "case", false)
	require.NoError(t, err)
	_, err = uc.PrefillAtRevision(ctx, "case", draft.Revision+1)
	require.ErrorIs(t, err, models.ConflictError)
	_, err = uc.ValidateDraft(ctx, "case", draft.Revision+1)
	require.ErrorIs(t, err, models.ConflictError)
	_, err = uc.PrefillAtRevision(ctx, "case", draft.Revision)
	require.NoError(t, err)
	_, err = uc.ValidateDraft(ctx, "case", draft.Revision)
	require.NoError(t, err)
}
