package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type draftTestRepo struct {
	config    models.TracfinConfiguration
	configErr error
	draft     models.TracfinDraft
	exists    bool
}

func (r *draftTestRepo) GetTracfinConfiguration(context.Context, repositories.Executor, string) (models.TracfinConfiguration, error) {
	return r.config, r.configErr
}
func (r *draftTestRepo) GetTracfinDraft(context.Context, repositories.Executor, string, string) (models.TracfinDraft, error) {
	if !r.exists {
		return models.TracfinDraft{}, models.NotFoundError
	}
	b, _ := json.Marshal(r.draft)
	var d models.TracfinDraft
	_ = json.Unmarshal(b, &d)
	return d, nil
}
func (r *draftTestRepo) GetTracfinDecisionSources(context.Context, repositories.Executor, string, string, []string) ([]models.TracfinDecisionSource, error) {
	return nil, nil
}
func (r *draftTestRepo) CreateTracfinDraft(_ context.Context, _ repositories.Executor, _, _ string, d models.TracfinDraft) error {
	if r.exists {
		return models.ConflictError
	}
	r.draft = d
	r.exists = true
	return nil
}
func (r *draftTestRepo) SaveTracfinDraft(_ context.Context, _ repositories.Executor, _, _ string, d models.TracfinDraft, rev int64) error {
	if rev != r.draft.Revision {
		return models.ConflictError
	}
	d.Revision = rev + 1
	r.draft = d
	return nil
}

type draftTestTransactions struct{}

func (draftTestTransactions) Transaction(ctx context.Context, fn func(repositories.Transaction) error) error {
	return fn(nil)
}
func draftTestContext(org uuid.UUID) context.Context {
	return context.WithValue(context.Background(), utils.ContextKeyCredentials, models.Credentials{OrganizationId: org, ActorIdentity: models.Identity{UserId: "actor", FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.org"}})
}
func draftTestUsecase(org uuid.UUID, repo *draftTestRepo, outcome models.CaseOutcome) TracfinUsecase {
	return NewTracfinUsecase(org, repo, tracfinCases{models.Case{Id: "case", OrganizationId: org, Outcome: outcome}}, nil, nil, tracfinExec{}, TracfinDraftDependencies{Repository: repo, Transactions: draftTestTransactions{}})
}
func findDraftField(b models.TracfinBlock, field string) []models.TracfinValue {
	if v := b.Fields[field]; len(v) > 0 {
		return v
	}
	for _, child := range b.Children {
		if v := findDraftField(child, field); len(v) > 0 {
			return v
		}
	}
	return nil
}

func TestTracfinDraftCreateManualAndDeclarant(t *testing.T) {
	for _, actor := range []bool{false, true} {
		t.Run(map[bool]string{false: "other", true: "actor"}[actor], func(t *testing.T) {
			org := uuid.New()
			repo := &draftTestRepo{configErr: models.NotFoundError}
			uc := draftTestUsecase(org, repo, models.CaseConfirmedRisk)
			got, err := uc.CreateDraft(draftTestContext(org), "case", actor)
			require.NoError(t, err)
			require.Equal(t, org.String(), got.OrgID)
			require.Equal(t, "case", got.CaseID)
			require.NotEmpty(t, got.Configuration.ProfileVersion)
			require.Empty(t, got.Configuration.Mappings)
			require.NotEmpty(t, got.Diagnostics)
			for field, want := range map[string]string{"/ds/declarant/coordonnees/prenom": "Ada", "/ds/declarant/coordonnees/nom": "Lovelace", "/ds/declarant/coordonnees/courriel": "ada@example.org"} {
				values := findDraftField(got.Document.Root, field)
				if actor {
					require.Len(t, values, 1)
					require.Equal(t, want, *values[0].Text)
				} else {
					require.Empty(t, values)
				}
			}
		})
	}
	org := uuid.New()
	repo := &draftTestRepo{configErr: models.NotFoundError}
	uc := draftTestUsecase(org, repo, models.CaseOutcomeUnset)
	_, err := uc.CreateDraft(draftTestContext(org), "case", true)
	require.Error(t, err)
	require.False(t, repo.exists)
}

func TestTracfinDraftTwoTabsAndServerBlockIDs(t *testing.T) {
	org := uuid.New()
	repo := &draftTestRepo{exists: true, draft: models.TracfinDraft{OrgID: org.String(), CaseID: "case", Revision: 4, Document: models.TracfinDocument{Root: models.TracfinBlock{ID: "ops", CatalogID: "/ds/operations"}}}}
	uc := draftTestUsecase(org, repo, models.CaseConfirmedRisk)
	ctx := draftTestContext(org)
	input := models.TracfinDraftInput{ExpectedRevision: 4, Changes: []models.TracfinChangeInput{{Action: "add_block", ParentBlockID: "ops", BlockID: "client-alias", CatalogID: "/ds/operations/operation"}}}
	got, err := uc.UpdateDraft(ctx, "case", input)
	require.NoError(t, err)
	require.Len(t, got.Document.Root.Children, 1)
	require.NotEqual(t, "client-alias", got.Document.Root.Children[0].ID)
	require.NotEmpty(t, got.Document.Root.Children[0].ID)
	_, err = uc.UpdateDraft(ctx, "case", input)
	require.ErrorIs(t, err, models.ConflictError)
	require.Len(t, repo.draft.Document.Root.Children, 1)
	require.Equal(t, got.Document.Root.Children[0].ID, repo.draft.Document.Root.Children[0].ID)
}
