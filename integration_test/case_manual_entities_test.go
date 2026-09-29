package integration

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCaseManualEntityRepository(t *testing.T) {
	ctx := context.Background()
	orgID, otherOrgID, inboxID, caseID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pgPool.Exec(ctx, `INSERT INTO organizations(id,name) VALUES ($1,$2),($3,$4)`, orgID, "manual-entities-"+orgID.String(), otherOrgID, "manual-entities-"+otherOrgID.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO inboxes(id,name,organization_id) VALUES ($1,'manual entities',$2)`, inboxID, orgID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, `INSERT INTO cases(id,org_id,inbox_id,name,type) VALUES ($1,$2,$3,'manual entities','decision')`, caseID, orgID, inboxID)
	require.NoError(t, err)

	exec, err := testUsecases.Repositories.ExecutorGetter.GetExecutor(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil)
	require.NoError(t, err)
	repo := repositories.NewMarbleDbRepository(false, 0.3)
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	results := make([]*models.CaseManualEntity, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = repo.InsertCaseManualEntity(ctx, exec, orgID, caseID.String(), ref)
		}(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, 1, boolToInt(results[0] != nil)+boolToInt(results[1] != nil))
	listed, err := repo.ListCaseManualEntities(ctx, exec, orgID, caseID.String())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, ref, listed[0].CaseEntityRef)
	require.NotEmpty(t, listed[0].Id)

	foreign, err := repo.InsertCaseManualEntity(ctx, exec, otherOrgID, caseID.String(), models.CaseEntityRef{TableName: "customers", ObjectId: "foreign"})
	require.NoError(t, err)
	require.Nil(t, foreign)
	foreignList, err := repo.ListCaseManualEntities(ctx, exec, otherOrgID, caseID.String())
	require.NoError(t, err)
	require.Empty(t, foreignList)
	foreignDelete, err := repo.DeleteCaseManualEntity(ctx, exec, otherOrgID, caseID.String(), ref)
	require.NoError(t, err)
	require.Nil(t, foreignDelete)

	deleted, err := repo.DeleteCaseManualEntity(ctx, exec, orgID, caseID.String(), ref)
	require.NoError(t, err)
	require.Equal(t, listed[0].Id, deleted.Id)
	require.Equal(t, ref, deleted.CaseEntityRef)
	again, err := repo.DeleteCaseManualEntity(ctx, exec, orgID, caseID.String(), ref)
	require.NoError(t, err)
	require.Nil(t, again)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestCaseEntityLookupWithoutCaption(t *testing.T) {
	e := httpexpect.Default(t, testServer.URL)
	admin, _ := setupOrgAndUser(e)
	tableResp := admin.POST("/data-model/tables").WithJSON(map[string]any{
		"name": "customers", "alias": "Customers", "description": "Customers without caption or index", "semantic_type": "person",
		"fields": []map[string]any{
			{"name": "object_id", "alias": "Object ID", "type": "String", "nullable": false},
			{"name": "updated_at", "alias": "Updated At", "type": "Timestamp", "nullable": false},
			{"name": "name", "alias": "Name", "type": "String", "semantic_type": "name"},
		},
	}).Expect()
	tableResp.Status(http.StatusCreated)
	apiKey := setupApiKey(e, admin)
	apiKey.POST("/v1/ingest/customers").WithJSON(map[string]any{
		"object_id": "c-123", "updated_at": "2026-09-29T00:00:00Z", "name": "Alice",
	}).Expect().Status(http.StatusCreated)
	admin.GET("/client_data/customers/c-123").Expect().Status(http.StatusOK).
		JSON().Object().Value("data").Object().Value("object_id").String().IsEqual("c-123")
}
