package integration

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newSarTestSelected(declarationId uuid.UUID, table, objectId string, createdAt time.Time) models.SarSelectedObject {
	decisionId := uuid.New()
	return models.SarSelectedObject{
		Id:            uuid.New(),
		DeclarationId: declarationId,
		TableName:     table,
		ObjectId:      objectId,
		Snapshot:      map[string]any{"object_id": objectId, "amount": float64(12)},
		Origin:        models.SarSnapshotOrigin{Kind: models.SarOriginKindDecision, DecisionId: &decisionId},
		CreatedAt:     createdAt,
	}
}

func TestSarSelection_AddListRemove(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	base := time.Now().Add(-time.Hour)
	a := newSarTestSelected(declaration.Id, "transactions", "t1", base.Add(2*time.Second))
	b := newSarTestSelected(declaration.Id, "accounts", "a1", base.Add(time.Second))
	validFrom := base
	b.Origin = models.SarSnapshotOrigin{Kind: models.SarOriginKindIngested, ValidFrom: &validFrom}
	b.ReachedFrom = &models.SarReachedFrom{TableName: "transactions", ObjectId: "t1", Link: "account"}
	require.NoError(t, repo.AddSarSelection(ctx, exec, []models.SarSelectedObject{a, b}))
	require.NoError(t, repo.AddSarSelection(ctx, exec, nil))

	list, err := repo.ListSarSelection(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "a1", list[0].ObjectId, "ordered by created_at")
	require.Equal(t, "t1", list[1].ObjectId)
	require.Equal(t, models.SarOriginKindIngested, list[0].Origin.Kind)
	require.NotNil(t, list[0].Origin.ValidFrom)
	require.Equal(t, b.ReachedFrom, list[0].ReachedFrom)
	require.Nil(t, list[1].ReachedFrom)
	require.Equal(t, a.Origin.DecisionId, list[1].Origin.DecisionId)
	require.Equal(t, a.Snapshot, list[1].Snapshot)

	require.NoError(t, repo.RemoveSarSelection(ctx, exec, declaration.Id,
		[]models.CaseEntityRef{{TableName: "accounts", ObjectId: "a1"}, {TableName: "nope", ObjectId: "x"}}))
	require.NoError(t, repo.RemoveSarSelection(ctx, exec, declaration.Id, nil))
	list, err = repo.ListSarSelection(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "t1", list[0].ObjectId)
}

func TestSarSelection_UniqueConflictWritesNothing(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))

	now := time.Now()
	require.NoError(t, repo.AddSarSelection(ctx, exec,
		[]models.SarSelectedObject{newSarTestSelected(declaration.Id, "transactions", "t1", now)}))
	err := repo.AddSarSelection(ctx, exec, []models.SarSelectedObject{
		newSarTestSelected(declaration.Id, "transactions", "t2", now),
		newSarTestSelected(declaration.Id, "transactions", "t1", now),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, models.ConflictError))
	list, err := repo.ListSarSelection(ctx, exec, declaration.Id)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestSarSelection_CascadeOnDeclarationDelete(t *testing.T) {
	ctx := t.Context()
	organizationID, caseID := sarDeclarationTestFixture(t)
	repo := repositories.NewMarbleDbRepository(false, 0)
	exec := testUsecases.NewExecutorFactory().NewExecutor()
	declaration := newSarTestDeclaration(organizationID, caseID)
	require.NoError(t, repo.CreateSarDeclaration(ctx, exec, declaration))
	require.NoError(t, repo.AddSarSelection(ctx, exec,
		[]models.SarSelectedObject{newSarTestSelected(declaration.Id, "transactions", "t1", time.Now())}))

	_, err := pgPool.Exec(ctx, "DELETE FROM sar_declarations WHERE id = $1", declaration.Id)
	require.NoError(t, err)
	var n int
	require.NoError(t, pgPool.QueryRow(ctx,
		"SELECT count(*) FROM sar_declaration_selections WHERE declaration_id = $1", declaration.Id).Scan(&n))
	require.Equal(t, 0, n)
}
