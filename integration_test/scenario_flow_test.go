package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/models"
	"marble/marble-backend/models/operators"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
	"os"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/exp/slog"
)

func TestScenarioEndToEnd(t *testing.T) {
	/*
		This test suite:
		- Creates an organization
		- Creates a user on the organization
		- Creates a data model for the organization
		- Creates a scenario on the organization
		- Creates a scenario iteration for the scenario, updates its body and publishes it
		- Ingests two accounts to be used in the scenario: one for a transaction to be rejected, one for a transaction to be approved
		- Creates a decision for the transaction to be rejected
		- Creates a decision for the transaction to be approved
		This corresponds to the critical path of our users.

		It does so by using the usecases (with credentials where applicable) and the repositories with a real local migrated docker db.
		It does not test the API layer.
	*/
	ctx := context.Background()

	// Initialize a logger and store it in the context
	textHandler := slog.HandlerOptions{ReplaceAttr: utils.LoggerAttributeReplacer}.NewTextHandler(os.Stderr)
	logger := slog.New(textHandler)
	ctx = utils.StoreLoggerInContext(ctx, logger)

	// Setup an organization and user credentials
	creds, dataModel := setupOrgAndCreds(ctx, t)
	organizationId := creds.OrganizationId
	ctx = context.WithValue(ctx, utils.ContextKeyCredentials, creds)

	// Now that we have a user and credentials, create a container for usecases with these credentials
	usecasesWithCreds := usecases.UsecasesWithCreds{
		Usecases:                testUsecases,
		Credentials:             creds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return organizationId, nil },
		Context:                 ctx,
	}

	// Scenario setup
	scenarioId := setupScenarioAndPublish(t, usecasesWithCreds, organizationId)

	// Ingest two accounts (parent of a transaction) to execute a full scenario: one to be rejected, one to be approved
	ingestAccounts(t, dataModel.Tables["accounts"], testUsecases, organizationId, logger)

	// Create a pair of decision and check that the outcome matches the expectation
	createDecisions(t, dataModel.Tables["transactions"], usecasesWithCreds, organizationId, scenarioId, logger)
}

func setupOrgAndCreds(ctx context.Context, t *testing.T) (models.Credentials, models.DataModel) {
	// Create a new organization
	orgUsecase := testUsecases.NewOrganizationUseCase()
	organization, err := orgUsecase.CreateOrganization(ctx, models.CreateOrganizationInput{
		Name:         "Test org n°42",
		DatabaseName: "test_org_42",
	})
	if err != nil {
		t.Fatalf("Could not create organization: %s", err)
	}
	organizationId := organization.ID
	fmt.Println("Created organization", organizationId)

	// Check that there are no users on the organization yet
	users, err := orgUsecase.GetUsersOfOrganization(organizationId)
	if err != nil {
		t.Fatalf("Could not get users of organization: %s", err)
	}
	if len(users) != 0 {
		t.Fatalf("Expected 0 users, got %d", len(users))
	}

	// Create a new admin user on the organization
	userUsecase := testUsecases.NewUserUseCase()
	adminUser, err := userUsecase.AddUser(models.CreateUser{
		Email:          "test@testmarble.com",
		OrganizationId: organizationId,
		Role:           models.ADMIN,
	})
	if err != nil {
		t.Fatalf("Could not create user: %s", err)
	}
	adminUserId := adminUser.UserId
	fmt.Println("Created admin user", adminUserId)

	// Create credentials for this user
	creds := models.Credentials{
		Role:           models.ADMIN,
		OrganizationId: organizationId,
		ActorIdentity: models.Identity{
			UserId: adminUserId,
		},
	}

	// Create a data model for the organization
	dataModel, err := orgUsecase.ReplaceDataModel(organizationId, newDataModel())
	if err != nil {
		t.Fatalf("Could not create data model: %s", err)
	}
	fmt.Println("Created data model")

	return creds, dataModel
}

func newDataModel() models.DataModel {
	return models.DataModel{
		Tables: map[models.TableName]models.Table{
			"transactions": {
				Name: "transactions",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at":  {DataType: models.Timestamp},
					"account_id":  {DataType: models.String, Nullable: true},
					"bic_country": {DataType: models.String, Nullable: true},
					"country":     {DataType: models.String, Nullable: true},
					"description": {DataType: models.String, Nullable: true},
					"direction":   {DataType: models.String, Nullable: true},
					"status":      {DataType: models.String, Nullable: true},
					"title":       {DataType: models.String, Nullable: true},
					"amount":      {DataType: models.Float, Nullable: true},
				},
				LinksToSingle: map[models.LinkName]models.LinkToSingle{
					"account": {
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id"},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at": {DataType: models.Timestamp},
					"balance":    {DataType: models.Float, Nullable: true},
					"company_id": {DataType: models.String, Nullable: true},
					"name":       {DataType: models.String, Nullable: true},
					"currency":   {DataType: models.String, Nullable: true},
					"is_frozen":  {DataType: models.Bool},
				},
				LinksToSingle: map[models.LinkName]models.LinkToSingle{
					"company": {
						LinkedTableName: "companies",
						ParentFieldName: "object_id",
						ChildFieldName:  "company_id"},
				},
			},
			"companies": {
				Name: "companies",
				Fields: map[models.FieldName]models.Field{
					"object_id": {
						DataType: models.String,
					},
					"updated_at": {DataType: models.Timestamp},
					"name":       {DataType: models.String, Nullable: true},
				},
			},
		},
	}
}

func setupScenarioAndPublish(t *testing.T, usecasesWithCreds usecases.UsecasesWithCreds, organizationId string) string {
	// Create a new empty scenario
	scenarioUsecase := usecasesWithCreds.NewScenarioUsecase()
	scenario, err := scenarioUsecase.CreateScenario(models.CreateScenarioInput{
		OrganizationID:    organizationId,
		Name:              "Test scenario",
		Description:       "Test scenario description",
		TriggerObjectType: "transactions",
	})
	if err != nil {
		t.Fatalf("Could not create scenario: %s", err)
	}
	scenarioId := scenario.ID
	fmt.Println("Created scenario", scenarioId)

	// Security: check that creating a scenario on the wrong organization fails
	_, err = scenarioUsecase.CreateScenario(models.CreateScenarioInput{
		OrganizationID:    uuid.New().String(),
		Name:              "Test scenario",
		Description:       "Test scenario description",
		TriggerObjectType: "transactions",
	})
	if err == nil {
		t.Fatalf("Expected error creating scenario on wrong organization, got nil")
	}

	// Now, create a scenario iteration, with a rule
	scenarioIterationUsecase := usecasesWithCreds.NewScenarioIterationUsecase()
	threshold := 10
	scenarioIteration, err := scenarioIterationUsecase.CreateScenarioIteration(usecasesWithCreds.Context, organizationId, models.CreateScenarioIterationInput{
		ScenarioID: scenarioId,
		Body: &models.CreateScenarioIterationBody{
			Rules: []models.CreateRuleInput{
				{
					Formula: &operators.EqString{
						Left:  &operators.DbFieldString{TriggerTableName: "transactions", FieldName: "name", Path: []string{"account"}},
						Right: &operators.StringValue{Value: "Reject test account"},
					},
					ScoreModifier: 100,
					Name:          "Check on account name",
					Description:   "Check on account name",
				},
			},
			TriggerCondition:     &operators.EqString{Left: &operators.StringValue{Value: "transactions"}, Right: &operators.StringValue{Value: "transactions"}},
			ScoreReviewThreshold: &threshold,
			ScoreRejectThreshold: &threshold,
			Schedule:             "*/10 * * * *",
		},
	})
	if err != nil {
		t.Fatalf("Could not create scenario iteration: %s", err)
	}
	scenarioIterationId := scenarioIteration.ID
	fmt.Println("Created scenario iteration", scenarioIterationId)

	// Actually, modify the scenario iteration
	threshold = 20
	updatedScenarioIteration, err := scenarioIterationUsecase.UpdateScenarioIteration(usecasesWithCreds.Context, organizationId, models.UpdateScenarioIterationInput{
		ID: scenarioIterationId,
		Body: &models.UpdateScenarioIterationBody{
			ScoreReviewThreshold: &threshold,
		},
	})
	if err != nil {
		t.Fatalf("Could not update scenario iteration: %s", err)
	}
	if *updatedScenarioIteration.Body.ScoreReviewThreshold != threshold {
		t.Fatalf("Expected score review threshold to be %d, got %d", threshold, *updatedScenarioIteration.Body.ScoreReviewThreshold)
	}

	// Publish the iteration to make it live
	scenarioPublicationUsecase := usecasesWithCreds.NewScenarioPublicationUsecase()
	scenarioPublications, err := scenarioPublicationUsecase.ExecuteScenarioPublicationAction(usecasesWithCreds.Context, models.PublishScenarioIterationInput{
		ScenarioIterationId: scenarioIterationId,
		PublicationAction:   models.Publish,
	})
	if err != nil {
		t.Fatalf("Could not publish scenario iteration: %s", err)
	}
	if len(scenarioPublications) != 1 {
		t.Fatalf("Expected 1 scenario publication, got %d", len(scenarioPublications))
	}
	fmt.Println("Published scenario iteration")

	// Now get the iteration and check it has a version
	scenarioIteration, err = scenarioIterationUsecase.GetScenarioIteration(usecasesWithCreds.Context, organizationId, scenarioIterationId)
	if err != nil {
		t.Fatalf("Could not get scenario iteration: %s", err)
	}
	if scenarioIteration.Version == nil || *scenarioIteration.Version != 1 {
		t.Fatal("Expected scenario iteration to have version 1\n")
	}
	fmt.Printf("Updated scenario iteration %+v\n", scenarioIteration.Body)

	return scenarioId
}

func ingestAccounts(t *testing.T, table models.Table, ussecases usecases.Usecases, organizationId string, logger *slog.Logger) {
	ingestionUsecase := testUsecases.NewIngestionUseCase()
	accountPayloadJson1 := []byte(`{
		"object_id": "{account_id_reject}",
		"updated_at": "2020-01-01T00:00:00Z",
		"name": "Reject test account"
	}`)
	accountPayloadJson2 := []byte(`{
		"object_id": "{account_id_approve}",
		"updated_at": "2020-01-01T00:00:00Z",
		"name": "Approve test account"
	}`)
	accountPayload1, err := app.ParseToDataModelObject(table, accountPayloadJson1)
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	accountPayload2, _ := app.ParseToDataModelObject(table, accountPayloadJson2)
	err = ingestionUsecase.IngestObjects(organizationId, []models.PayloadReader{accountPayload1, accountPayload2}, table, logger)
	if err != nil {
		t.Fatalf("Could not ingest data: %s", err)
	}
}

func createDecisions(t *testing.T, table models.Table, usecasesWithCreds usecases.UsecasesWithCreds, organizationId, scenarioId string, logger *slog.Logger) {
	decisionUsecase := testUsecases.NewDecisionUsecase()

	// Create a decision [REJECT]
	// First, create all the parts of the payload
	// TODO: refacto this usecase to move all of the business logic into the usecase
	transactionPayloadJson := []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_reject}",
		"amount": 100
	}`)
	triggerObjectMap := make(map[string]interface{})
	if err := json.Unmarshal(transactionPayloadJson, &triggerObjectMap); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err)
	}
	ClientObject := models.ClientObject{TableName: table.Name, Data: triggerObjectMap}
	transactionPayload, err := app.ParseToDataModelObject(table, transactionPayloadJson)
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	// Then, create the decision
	rejectDecision, err := decisionUsecase.CreateDecision(usecasesWithCreds.Context, models.CreateDecisionInput{
		ScenarioID:              scenarioId,
		ClientObject:            ClientObject,
		OrganizationID:          organizationId,
		PayloadStructWithReader: transactionPayload,
	}, logger)
	if err != nil {
		t.Fatalf("Could not create decision: %s", err)
	}
	if rejectDecision.Outcome != models.Reject {
		t.Fatalf("Expected decision to be Reject, got %s", rejectDecision.Outcome)
	}
	fmt.Println("Created decision", rejectDecision.DecisionId)

	// Create a decision [APROVE]
	// First, create all the parts of the payload
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}",
		"amount": 100
	}`)
	triggerObjectMap = make(map[string]interface{})
	if err := json.Unmarshal(transactionPayloadJson, &triggerObjectMap); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err)
	}
	ClientObject = models.ClientObject{TableName: table.Name, Data: triggerObjectMap}
	transactionPayload, err = app.ParseToDataModelObject(table, transactionPayloadJson)
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}
	// Then, create the decision
	approveDecision, err := decisionUsecase.CreateDecision(usecasesWithCreds.Context, models.CreateDecisionInput{
		ScenarioID:              scenarioId,
		ClientObject:            ClientObject,
		OrganizationID:          organizationId,
		PayloadStructWithReader: transactionPayload,
	}, logger)
	if err != nil {
		t.Fatalf("Could not create decision: %s", err)
	}
	if approveDecision.Outcome != models.Approve {
		t.Fatalf("Expected decision to be Approve, got %s", approveDecision.Outcome)
	}
	fmt.Println("Created decision", approveDecision.DecisionId)
}
