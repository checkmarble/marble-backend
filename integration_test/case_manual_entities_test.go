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
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	admin, _ := setupCaseEntityOrgAndUser(e)
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
	admin, _ := setupCaseEntityOrgAndUser(e)
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
	ctx = context.WithValue(ctx, utils.ContextKeyCredentials, withCreds.Credentials)
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
	// Two concurrent removals must produce exactly one event per effective deletion.
	var removalWG sync.WaitGroup
	removalErrors := make([]error, 2)
	for i := range removalErrors {
		removalWG.Add(1)
		go func(i int) {
			defer removalWG.Done()
			_, removalErrors[i] = uc.RemoveCaseEntities(ctx, userID, caseID, refs)
		}(i)
	}
	removalWG.Wait()
	for _, err := range removalErrors {
		require.NoError(t, err)
	}

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
	csCase := admin.POST("/cases").WithJSON(map[string]any{"inbox_id": inboxID, "name": "Continuous entities"}).Expect().Status(http.StatusCreated).JSON().Object().Value("case").Object().Value("id").String().Raw()
	_, err = pgPool.Exec(ctx, "UPDATE cases SET type='continuous_screening' WHERE id=$1", csCase)
	require.NoError(t, err)
	config, screening := uuid.New(), uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO continuous_screening_configs(id,org_id,inbox_id,name,algorithm,datasets,match_threshold,match_limit) VALUES($1,$2,$3,'Preserved','test','{}',50,10)", config, orgID, inboxID)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO continuous_screenings(id,org_id,case_id,continuous_screening_config_id,continuous_screening_config_stable_id,object_type,object_id,object_internal_id,trigger_type,search_input) VALUES($1,$2,$3,$4,$4,'customers','c-456',$5,'object_added','{}')", screening, orgID, csCase, config, uuid.New())
	require.NoError(t, err)
	csAdded, err := uc.AddCaseEntities(ctx, userID, csCase, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-456"}})
	require.NoError(t, err)
	require.Equal(t, models.CaseTypeContinuousScreening, csAdded.Type)
	require.Len(t, csAdded.ContinuousScreenings, 1)
	require.Equal(t, screening, csAdded.ContinuousScreenings[0].Id)
	csRemoved, err := uc.RemoveCaseEntities(ctx, userID, csCase, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-456"}})
	require.NoError(t, err)
	require.Len(t, csRemoved.ContinuousScreenings, 1)
	require.Equal(t, screening, csRemoved.ContinuousScreenings[0].Id)
	changed, err := pgPool.Exec(ctx, "UPDATE data_model_tables SET semantic_type='other' WHERE organization_id=$1 AND name='customers'", orgID)
	require.NoError(t, err)
	require.EqualValues(t, 1, changed.RowsAffected())
	var orgName string
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT name FROM organizations WHERE id=$1", orgID).Scan(&orgName))
	_, err = pgPool.Exec(ctx, "DELETE FROM "+pgx.Identifier{models.OrgSchemaName(orgName), "customers"}.Sanitize()+" WHERE object_id='c-123'")
	require.NoError(t, err)
	// Retrying an existing link is idempotent even after the table is no longer eligible.
	_, err = uc.AddCaseEntities(ctx, userID, caseID, []models.CaseEntityRef{{TableName: "customers", ObjectId: "c-123"}})
	require.NoError(t, err)

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

func TestCaseManualEntityRelatedCases(t *testing.T) {
	ctx := context.Background()
	org, inbox, manualCase, bothCase, decisionCase := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pgPool.Exec(ctx, "INSERT INTO organizations(id,name) VALUES($1,$2)", org, "related-"+org.String())
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO inboxes(id,name,organization_id) VALUES($1,'Related',$2)", inbox, org)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{manualCase, bothCase, decisionCase} {
		_, err = pgPool.Exec(ctx, "INSERT INTO cases(id,org_id,inbox_id,name,type,outcome) VALUES($1,$2,$3,'Related','decision','confirmed_risk')", id, org, inbox)
		require.NoError(t, err)
	}
	repo := repositories.NewMarbleDbRepository(false, 0.3)
	exec, err := testUsecases.Repositories.ExecutorGetter.GetExecutor(ctx, models.DATABASE_SCHEMA_TYPE_MARBLE, nil)
	require.NoError(t, err)
	ref := models.CaseEntityRef{TableName: "customers", ObjectId: "c-123"}
	for _, id := range []uuid.UUID{manualCase, bothCase} {
		_, err = repo.InsertCaseManualEntity(ctx, exec, org, id.String(), ref)
		require.NoError(t, err)
	}
	got, err := repo.GetCasesRelatedToObject(ctx, exec, org, "customers", "c-123")
	require.NoError(t, err)
	require.Len(t, got, 2)
	risk, err := repo.ObjectHasConfirmedRisks(ctx, exec, org, "customers", "c-123")
	require.NoError(t, err)
	require.False(t, risk)
	grouped, err := repo.SelectCasesWithPivot(ctx, exec, models.DecisionWorkflowFilters{OrganizationId: org, PivotValue: "c-123"})
	require.NoError(t, err)
	require.Empty(t, grouped)
	table, field, pivot := uuid.New(), uuid.New(), uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO data_model_tables(id,organization_id,name) VALUES($1,$2,'customers')", table, org)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO data_model_fields(id,table_id,name,type,nullable) VALUES($1,$2,'object_id','String',false)", field, table)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO data_model_pivots(id,organization_id,base_table_id,field_id) VALUES($1,$2,$3,$4)", pivot, org, table, field)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{bothCase, decisionCase} {
		_, err = pgPool.Exec(ctx, "INSERT INTO decisions(org_id,case_id,outcome,scenario_id,scenario_iteration_id,score,pivot_id,pivot_value) VALUES($1,$2,'approve',$3,$4,0,$5,'c-123')", org, id, uuid.New(), uuid.New(), pivot)
		require.NoError(t, err)
	}
	got, err = repo.GetCasesRelatedToObject(ctx, exec, org, "customers", "c-123")
	require.NoError(t, err)
	require.Len(t, got, 3)
	ids := make([]string, 0, len(got))
	for _, c := range got {
		ids = append(ids, c.Id)
	}
	require.ElementsMatch(t, []string{manualCase.String(), bothCase.String(), decisionCase.String()}, ids)
	risk, err = repo.ObjectHasConfirmedRisks(ctx, exec, org, "customers", "c-123")
	require.NoError(t, err)
	require.True(t, risk)
	grouped, err = repo.SelectCasesWithPivot(ctx, exec, models.DecisionWorkflowFilters{OrganizationId: org, PivotValue: "c-123"})
	require.NoError(t, err)
	require.Len(t, grouped, 2)
	foreign, err := repo.GetCasesRelatedToObject(ctx, exec, uuid.New(), "customers", "c-123")
	require.NoError(t, err)
	require.Empty(t, foreign)
}

func TestCaseManualEntitiesHTTP(t *testing.T) {
	ctx := context.Background()
	e := httpexpect.Default(t, testServer.URL)
	admin, viewer := setupCaseEntityOrgAndUser(e)
	admin.POST("/data-model/tables").WithJSON(map[string]any{"name": "customers", "alias": "Customers", "description": "No caption or index", "semantic_type": "person", "fields": []map[string]any{
		{"name": "object_id", "alias": "ID", "type": "String", "nullable": false},
		{"name": "updated_at", "alias": "Updated", "type": "Timestamp", "nullable": false},
		{"name": "name", "alias": "Name", "type": "String", "semantic_type": "name"},
	}}).Expect().Status(http.StatusCreated)
	key := setupApiKey(e, admin)
	for _, id := range []string{"c-123", "c-456"} {
		key.POST("/v1/ingest/customers").WithJSON(map[string]any{"object_id": id, "updated_at": "2026-09-29T00:00:00Z", "name": id}).Expect().Status(http.StatusCreated)
	}
	inbox := admin.POST("/inboxes").WithJSON(map[string]any{"name": "Entities"}).Expect().Status(http.StatusOK).JSON().Object().Value("inbox").Object().Value("id").String().Raw()
	refs := []map[string]any{{"table_name": "customers", "object_id": "c-123"}}
	created := admin.POST("/cases").WithJSON(map[string]any{"inbox_id": inbox, "name": "Entities", "entities": refs}).Expect().Status(http.StatusCreated).JSON().Object().Value("case").Object()
	id := created.Value("id").String().Raw()
	created.Value("decisions_count").Number().IsEqual(0)
	entities := created.Value("entities").Array()
	entities.Length().IsEqual(1)
	entities.Value(0).Object().Value("sources").Array().ContainsOnly("manual")
	entities.Value(0).Object().Value("data").Object().Value("object_id").String().IsEqual("c-123")
	var entityLinkID string
	for _, raw := range created.Value("events").Array().Raw() {
		event := raw.(map[string]any)
		if event["event_type"] == "entity_added" {
			require.Equal(t, "case_manual_entity", event["resource_type"])
			require.JSONEq(t, `{"table_name":"customers","object_id":"c-123"}`, event["new_value"].(string))
			entityLinkID = event["resource_id"].(string)
			_, err := uuid.Parse(entityLinkID)
			require.NoError(t, err)
		}
	}
	require.NotEmpty(t, entityLinkID)
	var org uuid.UUID
	var user, orgName string
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT c.org_id,o.name,c.assigned_to::text FROM cases c JOIN organizations o ON o.id=c.org_id WHERE c.id=$1", id).Scan(&org, &orgName, &user))
	scenario, iteration, pivot, decision := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pgPool.Exec(ctx, "INSERT INTO scenarios(id,org_id,name,description,trigger_object_type) VALUES($1,$2,'Entities scenario','','customers')", scenario, org)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO scenario_iterations(id,org_id,scenario_id,version) VALUES($1,$2,$3,1)", iteration, org, scenario)
	require.NoError(t, err)
	err = pgPool.QueryRow(ctx, "SELECT p.id FROM data_model_pivots p JOIN data_model_tables t ON t.id=p.base_table_id JOIN data_model_fields f ON f.id=p.field_id WHERE t.organization_id=$1 AND t.name='customers' AND f.name='object_id' AND p.deleted_at IS NULL", org).Scan(&pivot)
	require.NoError(t, err)
	_, err = pgPool.Exec(ctx, "INSERT INTO decisions(id,org_id,case_id,outcome,scenario_id,scenario_iteration_id,score,pivot_id,pivot_value,trigger_object_type,trigger_object) VALUES($1,$2,$3,'approve',$4,$5,0,$6,'c-123','customers','{}')", decision, org, id, scenario, iteration, pivot)
	require.NoError(t, err)
	merged := admin.GET("/cases/{id}", id).Expect().Status(http.StatusOK).JSON().Object()
	merged.Value("entities").Array().Length().IsEqual(1)
	merged.Value("entities").Array().Value(0).Object().Value("sources").Array().ContainsOnly("manual", "decision")
	oldDecision := uuid.New()
	_, err = pgPool.Exec(ctx, "INSERT INTO decisions(id,org_id,outcome,scenario_id,scenario_iteration_id,score,pivot_id,pivot_value,trigger_object_type,trigger_object) VALUES($1,$2,'approve',$3,$4,0,$5,'c-456','customers','{}')", oldDecision, org, scenario, iteration, pivot)
	require.NoError(t, err)
	legacyID := admin.POST("/cases").WithJSON(map[string]any{"inbox_id": inbox, "name": "With decisions", "decision_ids": []string{oldDecision.String()}}).Expect().Status(http.StatusCreated).JSON().Object().Value("case").Object().Value("id").String().Raw()
	admin.GET("/cases/{id}/pivot_objects", legacyID).Expect().Status(http.StatusOK).JSON().Object().Value("pivot_objects").Array().Length().IsEqual(1)
	admin.POST("/cases/{id}/entities", id).WithJSON(map[string]any{"entities": []map[string]any{{"table_name": "customers", "object_id": "c-123"}, {"table_name": "customers", "object_id": "c-456"}}}).Expect().Status(http.StatusOK).JSON().Object().Value("case").Object().Value("entities").Array().Length().IsEqual(2)
	admin.PATCH("/cases/{id}", id).WithJSON(map[string]any{"name": "Updated"}).Expect().Status(http.StatusOK).JSON().Object().Value("case").Object().Value("entities").Array().Length().IsEqual(2)
	for _, body := range []any{map[string]any{}, map[string]any{"entities": []any{}}, map[string]any{"entities": []map[string]any{{"table_name": "customers"}}}, map[string]any{"entities": []map[string]any{refs[0], refs[0]}}} {
		admin.POST("/cases/{id}/entities", id).WithJSON(body).Expect().Status(http.StatusBadRequest)
		admin.DELETE("/cases/{id}/entities", id).WithJSON(body).Expect().Status(http.StatusBadRequest)
	}
	admin.POST("/cases/{id}/entities", id).WithBytes([]byte("{broken")).WithHeader("Content-Type", "application/json").Expect().Status(http.StatusBadRequest)
	for _, ref := range []map[string]any{{"table_name": "customers", "object_id": "absent"}, {"table_name": "missing", "object_id": "c-123"}} {
		admin.POST("/cases/{id}/entities", id).WithJSON(map[string]any{"entities": []map[string]any{ref}}).Expect().Status(http.StatusUnprocessableEntity)
	}
	viewer.POST("/cases/{id}/entities", id).WithJSON(map[string]any{"entities": refs}).Expect().Status(http.StatusForbidden)
	creds := generateUsecaseWithCreds(testUsecases, models.Credentials{OrganizationId: org, Role: models.ANALYST, ActorIdentity: models.Identity{UserId: models.UserId(user)}})
	// A CASE_READ_WRITE principal without inbox membership is denied within the same organization.
	_, err = pgPool.Exec(ctx, "DELETE FROM inbox_users WHERE user_id=$1", user)
	require.NoError(t, err)
	_, err = creds.NewCaseUseCase().AddCaseEntities(ctx, user, id, refsToModels(refs))
	require.Error(t, err)
	_, err = creds.NewCaseUseCase().RemoveCaseEntities(ctx, user, id, refsToModels(refs))
	require.Error(t, err)
	_, err = creds.NewCaseUseCase().GetCaseWithEntities(ctx, id, true)
	require.Error(t, err)
	// Historical inactive objects keep their identity with null data and cannot form new links.
	_, err = pgPool.Exec(ctx, "UPDATE "+pgx.Identifier{models.OrgSchemaName(orgName), "customers"}.Sanitize()+" SET valid_until=now() WHERE object_id='c-456'")
	require.NoError(t, err)
	historical := admin.GET("/cases/{id}", id).Expect().Status(http.StatusOK).JSON().Object().Value("entities").Array()
	historical.Length().IsEqual(2)
	historical.Value(1).Object().Value("data").IsNull()
	admin.POST("/cases").WithJSON(map[string]any{"inbox_id": inbox, "name": "Inactive rejected", "entities": []map[string]any{{"table_name": "customers", "object_id": "c-456"}}}).Expect().Status(http.StatusUnprocessableEntity)
	// Renaming the physical ClientDB table forces only hydration to fail after a committed retry.
	physical := pgx.Identifier{models.OrgSchemaName(orgName), "customers"}.Sanitize()
	_, err = pgPool.Exec(ctx, "ALTER TABLE "+physical+" RENAME TO customers_unavailable")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pgPool.Exec(context.Background(), "ALTER TABLE "+pgx.Identifier{models.OrgSchemaName(orgName), "customers_unavailable"}.Sanitize()+" RENAME TO customers")
	})
	response := admin.POST("/cases/{id}/entities", id).WithJSON(map[string]any{"entities": refs}).Expect().Status(http.StatusOK).JSON().Object().Value("case").Object()
	response.Value("entities").Array().Value(0).Object().Value("data").IsNull()
	admin.GET("/cases/{id}", id).Expect().Status(http.StatusInternalServerError)
	removed := admin.DELETE("/cases/{id}/entities", id).WithJSON(map[string]any{"entities": refs}).Expect().Status(http.StatusOK).JSON().Object().Value("case").Object()
	removed.Value("entities").Array().Length().IsEqual(2)
	removed.Value("entities").Array().Value(0).Object().Value("sources").Array().ContainsOnly("decision")
	removed.Value("decisions").Array().Length().IsEqual(1)
	events := removed.Value("events").Array().Raw()
	sawRemove := false
	for _, value := range events {
		event := value.(map[string]any)
		if event["event_type"] != "entity_removed" {
			continue
		}
		sawRemove = true
		require.Equal(t, "case_manual_entity", event["resource_type"])
		require.Equal(t, user, event["user_id"])
		require.Equal(t, entityLinkID, event["resource_id"])
		_, err := uuid.Parse(event["resource_id"].(string))
		require.NoError(t, err)
		require.JSONEq(t, `{"table_name":"customers","object_id":"c-123"}`, event["previous_value"].(string))
	}
	require.True(t, sawRemove)
}
func refsToModels(refs []map[string]any) []models.CaseEntityRef {
	result := make([]models.CaseEntityRef, len(refs))
	for i, ref := range refs {
		result[i] = models.CaseEntityRef{TableName: ref["table_name"].(string), ObjectId: ref["object_id"].(string)}
	}
	return result
}

// The existing end-to-end helper assumes an empty database and fixed emails.
// These tests run together, so every fixture owns a separate organization and identities.
func setupCaseEntityOrgAndUser(e *httpexpect.Expect) (*httpexpect.Expect, *httpexpect.Expect) {
	token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(marbleAdminEmail)).Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
	marbleAdmin := e.Builder(func(req *httpexpect.Request) { req.WithHeader("Authorization", "Bearer "+token) })
	org := marbleAdmin.POST("/organizations").WithJSON(map[string]any{"name": "case-entities-" + uuid.NewString()}).Expect().Status(http.StatusOK).JSON().Object().Value("organization").Object().Value("id").String().Raw()
	clients := make([]*httpexpect.Expect, 0, 2)
	for _, role := range []string{"ADMIN", "VIEWER"} {
		email := uuid.NewString() + "@case-entities.test"
		marbleAdmin.POST("/users").WithJSON(map[string]any{"email": email, "organization_id": org, "role": role}).Expect().Status(http.StatusOK)
		token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(email)).Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
		clients = append(clients, e.Builder(func(req *httpexpect.Request) { req.WithHeader("Authorization", "Bearer "+token) }))
	}
	return clients[0], clients[1]
}
