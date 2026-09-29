package integration

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

func TestCaseManualEntitiesMutation(t *testing.T) {
	ctx := context.Background()
	e := httpexpect.Default(t, testServer.URL)
	admin, _ := setupOrgAndUser(e)
	admin.POST("/data-model/tables").WithJSON(map[string]any{
		"name": "customers", "alias": "Customers", "description": "Customers", "semantic_type": "person",
		"fields": []map[string]any{
			{"name": "object_id", "alias": "Object ID", "type": "String", "nullable": false},
			{"name": "updated_at", "alias": "Updated At", "type": "Timestamp", "nullable": false},
			{"name": "name", "alias": "Name", "type": "String", "semantic_type": "name"},
		},
	}).Expect().Status(http.StatusCreated)
	apiKey := setupApiKey(e, admin)
	for _, id := range []string{"c-123", "c-456"} {
		apiKey.POST("/v1/ingest/customers").WithJSON(map[string]any{
			"object_id": id, "updated_at": "2026-09-29T00:00:00Z", "name": id,
		}).Expect().Status(http.StatusCreated)
	}
	inboxID := admin.POST("/inboxes").WithJSON(map[string]any{"name": "Manual entities"}).
		Expect().Status(http.StatusOK).JSON().Object().Value("inbox").Object().Value("id").String().Raw()
	caseID := admin.POST("/cases").WithJSON(map[string]any{"inbox_id": inboxID, "name": "Manual entities"}).
		Expect().Status(http.StatusCreated).JSON().Object().Value("case").Object().Value("id").String().Raw()
	var orgID uuid.UUID
	err := pgPool.QueryRow(ctx, "SELECT org_id FROM cases WHERE id=$1", caseID).Scan(&orgID)
	require.NoError(t, err)
	var userID string
	err = pgPool.QueryRow(ctx, "SELECT id::text FROM users WHERE organization_id=$1 AND role=$2 LIMIT 1", orgID, int(models.ADMIN)).Scan(&userID)
	require.NoError(t, err)
	withCreds := generateUsecaseWithCreds(testUsecases, models.Credentials{OrganizationId: orgID, Role: models.ADMIN, ActorIdentity: models.Identity{UserId: models.UserId(userID)}})
	uc := withCreds.NewCaseUseCase()
	refs := []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}, {TableName: "customers", ObjectId: "c-456"}}
	parsedInboxID, err := uuid.Parse(inboxID)
	require.NoError(t, err)
	created, err := uc.CreateCaseAsUser(ctx, orgID, userID, models.CreateCaseAttributes{
		OrganizationId: orgID, InboxId: parsedInboxID, Name: "Created with entities", Type: models.CaseTypeDecision, Entities: refs,
	})
	require.NoError(t, err)
	require.Zero(t, created.DecisionsCount)
	var createdLinks int
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_manual_entities WHERE case_id=$1", created.Id).Scan(&createdLinks)
	require.NoError(t, err)
	require.Equal(t, 2, createdLinks)
	otherCreds := generateUsecaseWithCreds(testUsecases, models.Credentials{OrganizationId: uuid.New(), Role: models.ADMIN})
	_, err = otherCreds.NewCaseUseCase().AddCaseEntities(ctx, "", caseID, refs)
	require.Error(t, err)
	viewerCreds := generateUsecaseWithCreds(testUsecases, models.Credentials{OrganizationId: orgID, Role: models.VIEWER})
	_, err = viewerCreds.NewCaseUseCase().AddCaseEntities(ctx, "", caseID, refs)
	require.Error(t, err)
	_, err = uc.AddCaseEntities(ctx, userID, caseID, refs)
	require.NoError(t, err)
	_, err = uc.AddCaseEntities(ctx, userID, caseID, refs)
	require.NoError(t, err)
	var added int
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_events WHERE case_id=$1 AND event_type='entity_added'", caseID).Scan(&added)
	require.NoError(t, err)
	require.Equal(t, 2, added)
	_, err = uc.RemoveCaseEntities(ctx, userID, caseID, refs)
	require.NoError(t, err)
	_, err = uc.RemoveCaseEntities(ctx, userID, caseID, refs)
	require.NoError(t, err)
	var removed int
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_events WHERE case_id=$1 AND event_type='entity_removed'", caseID).Scan(&removed)
	require.NoError(t, err)
	require.Equal(t, 2, removed)
	var links int
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_manual_entities WHERE case_id=$1", caseID).Scan(&links)
	require.NoError(t, err)
	require.Zero(t, links)
	_, err = uc.AddCaseEntities(ctx, userID, caseID, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}, {TableName: "customers", ObjectId: "absent"}})
	require.Error(t, err)
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_manual_entities WHERE case_id=$1", caseID).Scan(&links)
	require.NoError(t, err)
	require.Zero(t, links)
	_, err = uc.AddCaseEntities(ctx, userID, caseID, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}})
	require.NoError(t, err)
	changed, err := pgPool.Exec(ctx, "UPDATE data_model_tables SET semantic_type='other' WHERE organization_id=$1 AND name='customers'", orgID)
	require.NoError(t, err)
	require.EqualValues(t, 1, changed.RowsAffected())
	_, err = uc.RemoveCaseEntities(ctx, userID, caseID, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}})
	require.NoError(t, err)

	// An event failure must roll back the link inserted earlier in the transaction.
	_, err = pgPool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION marble.fail_case_entity_event_%s() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN IF NEW.case_id = '%s' AND NEW.event_type = 'entity_added' THEN RAISE EXCEPTION 'forced event failure'; END IF; RETURN NEW; END $$`, strings.ReplaceAll(caseID, "-", ""), caseID))
	require.NoError(t, err)
	triggerName := "fail_case_entity_event_" + strings.ReplaceAll(caseID, "-", "")
	_, err = pgPool.Exec(ctx, fmt.Sprintf("CREATE TRIGGER %s BEFORE INSERT ON case_events FOR EACH ROW EXECUTE FUNCTION marble.%s()", triggerName, triggerName))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pgPool.Exec(context.Background(), "DROP TRIGGER IF EXISTS "+triggerName+" ON case_events")
		_, _ = pgPool.Exec(context.Background(), "DROP FUNCTION IF EXISTS marble."+triggerName+"()")
	})
	// Retyping above makes addition invalid, so use a different table-free path to hit the event trigger:
	// restore the category after verifying historical removal.
	_, err = pgPool.Exec(ctx, "UPDATE data_model_tables SET semantic_type='person' WHERE organization_id=$1 AND name='customers'", orgID)
	require.NoError(t, err)
	_, err = uc.AddCaseEntities(ctx, userID, caseID, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-456"}})
	require.Error(t, err)
	err = pgPool.QueryRow(ctx, "SELECT count(*) FROM case_manual_entities WHERE case_id=$1", caseID).Scan(&links)
	require.NoError(t, err)
	require.Zero(t, links)
}
