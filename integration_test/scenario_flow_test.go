package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/segmentio/analytics-go/v3"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/utils"
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
	logHandler := utils.LocalDevHandlerOptions{
		SlogOpts: slog.HandlerOptions{Level: slog.LevelDebug},
		UseColor: true,
	}.NewLocalDevHandler(os.Stdout)
	logger := slog.New(logHandler)

	ctx = utils.StoreLoggerInContext(ctx, logger)
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

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
	scenarioId := setupScenarioAndPublish(t, ctx, usecasesWithCreds, organizationId)

	apiCreds := setupApiCreds(ctx, t, usecasesWithCreds, organizationId)
	usecasesWithApiCreds := usecases.UsecasesWithCreds{
		Usecases:                testUsecases,
		Credentials:             apiCreds,
		Logger:                  utils.LoggerFromContext(ctx),
		OrganizationIdOfContext: func() (string, error) { return organizationId, nil },
		Context:                 ctx,
	}

	// Ingest two accounts (parent of a transaction) to execute a full scenario: one to be rejected, one to be approved
	ingestAccounts(t, dataModel.Tables["accounts"], usecasesWithApiCreds, organizationId, logger)

	// Create a pair of decision and check that the outcome matches the expectation
	createDecisions(t, dataModel.Tables["transactions"], usecasesWithApiCreds, organizationId, scenarioId, logger)
}

func setupApiCreds(ctx context.Context, t *testing.T, usecasesWithCreds usecases.UsecasesWithCreds, organizationId string) models.Credentials {
	// Create an API Key for this org
	apiKeyUsecase := usecasesWithCreds.NewApiKeyUseCase()
	apiKey, err := apiKeyUsecase.CreateApiKey(ctx, models.CreateApiKeyInput{
		OrganizationId: organizationId,
		Description:    "Test API key",
		Role:           models.API_CLIENT,
	})
	assert.NoError(t, err, "Could not create api key")

	// Generate creds from the created API Key
	marbleTokenUsecase := usecasesWithCreds.NewMarbleTokenUseCase()
	creds, err := marbleTokenUsecase.ValidateCredentials(ctx, "", apiKey.Hash)
	assert.NoError(t, err, "Could not generate creds from api key")
	return creds
}

func setupOrgAndCreds(ctx context.Context, t *testing.T) (models.Credentials, models.DataModel) {
	// Create a new organization
	testAdminUsecase := GenerateUsecaseWithCredForMarbleAdmin(ctx, testUsecases)
	orgUsecase := testAdminUsecase.NewOrganizationUseCase()
	organization, err := orgUsecase.CreateOrganization(ctx, models.CreateOrganizationInput{
		Name:         "Test org n°42",
		DatabaseName: "test_org_42",
	})
	assert.NoError(t, err, "Could not create organization")
	organizationId := organization.Id
	fmt.Println("Created organization", organizationId)

	// Check that there are no users on the organization yet
	users, err := orgUsecase.GetUsersOfOrganization(ctx, organizationId)
	assert.NoError(t, err, "Could not get users of organization")
	assert.Equal(t, 0, len(users), "Expected 0 users, got %d", len(users))

	// Create a new admin user on the organization
	userUsecase := testAdminUsecase.NewUserUseCase()
	adminUser, err := userUsecase.AddUser(ctx, models.CreateUser{
		Email:          "test@testmarble.com",
		OrganizationId: organizationId,
		Role:           models.ADMIN,
	})
	assert.NoError(t, err, "Could not create user")
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
	dataModel, err := createDataModel(t, organizationId)
	assert.NoError(t, err, "Could not create data model")
	fmt.Println("Created data model")

	return creds, dataModel
}

func createDataModel(t *testing.T, organizationID string) (models.DataModel, error) {
	testAdminUsecase := GenerateUsecaseWithCredForMarbleAdmin(context.Background(), testUsecases)
	ctx := context.TODO()

	usecase := testAdminUsecase.NewDataModelUseCase()
	transactionsTableID, err := usecase.CreateDataModelTable(ctx, organizationID, "transactions", "description")
	assert.NoError(t, err)
	transactionsFields := []models.DataModelField{
		{Name: "account_id", Type: models.String.String(), Nullable: true},
		{Name: "bic_country", Type: models.String.String(), Nullable: true},
		{Name: "country", Type: models.String.String(), Nullable: true},
		{Name: "description", Type: models.String.String(), Nullable: true},
		{Name: "direction", Type: models.String.String(), Nullable: true},
		{Name: "status", Type: models.String.String(), Nullable: true},
		{Name: "title", Type: models.String.String(), Nullable: true},
		{Name: "amount", Type: models.Float.String(), Nullable: true},
	}
	for _, field := range transactionsFields {
		_, err = usecase.CreateDataModelField(ctx, transactionsTableID, field)
		assert.NoError(t, err)
	}

	accountsTableID, err := usecase.CreateDataModelTable(ctx, organizationID, "accounts", "description")
	assert.NoError(t, err)
	accountsFields := []models.DataModelField{
		{Name: "balance", Type: models.Float.String(), Nullable: true},
		{Name: "company_id", Type: models.String.String(), Nullable: true},
		{Name: "name", Type: models.String.String(), Nullable: true},
		{Name: "currency", Type: models.String.String(), Nullable: true},
		{Name: "is_frozen", Type: models.Bool.String(), Nullable: true},
	}
	for _, field := range accountsFields {
		_, err = usecase.CreateDataModelField(ctx, accountsTableID, field)
		assert.NoError(t, err)
	}

	companiesTableID, err := usecase.CreateDataModelTable(ctx, organizationID, "companies", "description")
	assert.NoError(t, err)
	companiesFields := []models.DataModelField{
		{Name: "name", Type: models.Float.String(), Nullable: true},
	}
	for _, field := range companiesFields {
		_, err = usecase.CreateDataModelField(ctx, companiesTableID, field)
		assert.NoError(t, err)
	}

	dm, err := usecase.GetDataModel(ctx, organizationID)
	assert.NoError(t, err)

	err = usecase.CreateDataModelLink(ctx, models.DataModelLink{
		Name:           "account",
		OrganizationID: organizationID,
		ParentTableID:  accountsTableID,
		ParentFieldID:  dm.Tables["accounts"].Fields["object_id"].ID,
		ChildTableID:   transactionsTableID,
		ChildFieldID:   dm.Tables["transactions"].Fields["account_id"].ID,
	})
	assert.NoError(t, err)

	err = usecase.CreateDataModelLink(ctx, models.DataModelLink{
		Name:           "company",
		OrganizationID: organizationID,
		ParentTableID:  companiesTableID,
		ParentFieldID:  dm.Tables["companies"].Fields["object_id"].ID,
		ChildTableID:   accountsTableID,
		ChildFieldID:   dm.Tables["accounts"].Fields["company_id"].ID,
	})
	assert.NoError(t, err)
	return usecase.GetDataModel(ctx, organizationID)
}

func setupScenarioAndPublish(t *testing.T, ctx context.Context,
	usecasesWithCreds usecases.UsecasesWithCreds, organizationId string,
) string {
	// Create a new empty scenario
	scenarioUsecase := usecasesWithCreds.NewScenarioUsecase()
	scenario, err := scenarioUsecase.CreateScenario(ctx, models.CreateScenarioInput{
		Name:              "Test scenario",
		Description:       "Test scenario description",
		TriggerObjectType: "transactions",
	})
	assert.NoError(t, err, "Could not create scenario")
	scenarioId := scenario.Id
	fmt.Println("Created scenario", scenarioId)

	assert.Equal(t, scenario.OrganizationId, organizationId)

	// Now, create a scenario iteration, with a rule
	scenarioIterationUsecase := usecasesWithCreds.NewScenarioIterationUsecase()
	threshold := 20
	scenarioIteration, err := scenarioIterationUsecase.CreateScenarioIteration(
		usecasesWithCreds.Context, organizationId, models.CreateScenarioIterationInput{
			ScenarioId: scenarioId,
			Body: &models.CreateScenarioIterationBody{
				Rules: []models.CreateRuleInput{
					{
						FormulaAstExpression: &ast.Node{
							Function: ast.FUNC_AND,
							Children: []ast.Node{
								{
									Function: ast.FUNC_EQUAL,
									Children: []ast.Node{
										{
											Function: ast.FUNC_DB_ACCESS,
											NamedChildren: map[string]ast.Node{
												"tableName": {Constant: "transactions"},
												"fieldName": {Constant: "name"},
												"path":      {Constant: []string{"account"}},
											},
										},
										{Constant: "Reject test account"},
									},
								},
								{
									Function: ast.FUNC_EQUAL,
									Children: []ast.Node{
										{Constant: 1},
										{
											Function: ast.FUNC_DIVIDE,
											Children: []ast.Node{
												{Constant: 100},
												{
													Function: ast.FUNC_PAYLOAD,
													Children: []ast.Node{
														{Constant: "amount"},
													},
												},
											},
										},
									},
								},
							},
						},
						ScoreModifier: 100,
						Name:          "Check on account name",
						Description:   "Check on account name",
					},
					{
						FormulaAstExpression: &ast.Node{
							Function: ast.FUNC_GREATER,
							Children: []ast.Node{
								{Constant: 500},
								{
									Function: ast.FUNC_AGGREGATOR,
									NamedChildren: map[string]ast.Node{
										"tableName":  {Constant: "transactions"},
										"fieldName":  {Constant: "amount"},
										"aggregator": {Constant: ast.AGGREGATOR_SUM},
										"label":      {Constant: "An aggregator function"},
										"filters": {
											Function: ast.FUNC_LIST,
											Children: []ast.Node{
												{
													Function: ast.FUNC_FILTER,
													NamedChildren: map[string]ast.Node{
														"tableName": {Constant: "transactions"},
														"fieldName": {Constant: "amount"},
														"operator":  {Constant: ast.FILTER_EQUAL},
														"value": {
															Function: ast.FUNC_PAYLOAD,
															Children: []ast.Node{
																{Constant: "amount"},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
						ScoreModifier: 10,
						Name:          "Check on aggregated value",
						Description:   "Check on aggregated value",
					},
				},
				TriggerConditionAstExpression: &ast.Node{
					Function: ast.FUNC_EQUAL,
					Children: []ast.Node{{Constant: "transactions"}, {Constant: "transactions"}},
				},
				ScoreReviewThreshold: &threshold,
				ScoreRejectThreshold: &threshold,
				Schedule:             "*/10 * * * *",
			},
		})
	assert.NoError(t, err, "Could not create scenario iteration")
	scenarioIterationId := scenarioIteration.Id
	fmt.Println("Created scenario iteration", scenarioIterationId)

	// Actually, modify the scenario iteration
	threshold = 30
	updatedScenarioIteration, err := scenarioIterationUsecase.UpdateScenarioIteration(
		usecasesWithCreds.Context, organizationId, models.UpdateScenarioIterationInput{
			Id: scenarioIterationId,
			Body: &models.UpdateScenarioIterationBody{
				ScoreRejectThreshold: &threshold,
			},
		})
	assert.NoError(t, err)

	validation, err := scenarioIterationUsecase.ValidateScenarioIteration(ctx, scenarioIterationId, nil, nil)
	assert.NoError(t, err)

	assert.NoError(t, scenarios.ScenarioValidationToError(validation))
	assert.NoError(t, err, "Could not update scenario iteration")

	if assert.NotNil(t, updatedScenarioIteration.ScoreRejectThreshold) {
		assert.Equal(
			t,
			threshold, *updatedScenarioIteration.ScoreRejectThreshold,
			"Expected score review threshold to be %d, got %d", threshold,
			*updatedScenarioIteration.ScoreRejectThreshold,
		)
	}

	// Publish the iteration to make it live
	scenarioPublicationUsecase := usecasesWithCreds.NewScenarioPublicationUsecase()
	_, err = scenarioIterationUsecase.CommitScenarioIterationVersion(ctx, scenarioIterationId)
	assert.NoError(t, err, "Could not commit scenario iteration")
	err = scenarioPublicationUsecase.StartPublicationPreparation(ctx, scenarioIterationId)
	assert.NoError(t, err, "Could not start publication preparation")
	time.Sleep(50 * time.Millisecond)
	scenarioPublications, err := scenarioPublicationUsecase.ExecuteScenarioPublicationAction(
		ctx, models.PublishScenarioIterationInput{
			ScenarioIterationId: scenarioIterationId,
			PublicationAction:   models.Publish,
		})
	assert.NoError(t, err, "Could not publish scenario iteration")
	assert.Equal(t, 1, len(scenarioPublications), "Expected 1 scenario publication, got %d", len(scenarioPublications))
	fmt.Println("Published scenario iteration")

	// Now get the iteration and check it has a version
	scenarioIteration, err = scenarioIterationUsecase.GetScenarioIteration(ctx, scenarioIterationId)
	assert.NoError(t, err, "Could not get scenario iteration")

	assert.NotNil(t, scenarioIteration.Version, "Expected scenario iteration to have a version")
	if assert.NotNil(t, scenarioIteration.Version) {
		assert.Equal(t, 1, *scenarioIteration.Version,
			"Expected scenario iteration to have version")
	}
	fmt.Printf("Updated scenario iteration %+v\n", scenarioIteration)

	return scenarioId
}

func ingestAccounts(t *testing.T, table models.Table, usecases usecases.UsecasesWithCreds, organizationId string, logger *slog.Logger) {
	ingestionUsecase := usecases.NewIngestionUseCase()
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
	accountPayloadJson3 := []byte(`{
		"object_id": "{account_id_approve_no_name}",
		"updated_at": "2020-01-01T00:00:00Z"
	}`)
	accountPayload1, err := payload_parser.ParseToDataModelObject(table, accountPayloadJson1)
	assert.NoError(t, err, "Could not parse payload")
	accountPayload2, _ := payload_parser.ParseToDataModelObject(table, accountPayloadJson2)
	accountPayload3, _ := payload_parser.ParseToDataModelObject(table, accountPayloadJson3)
	err = ingestionUsecase.IngestObjects(context.TODO(), organizationId, []models.PayloadReader{
		accountPayload1, accountPayload2, accountPayload3,
	}, table, logger)
	assert.NoError(t, err, "Could not ingest data")
}

func createTransactionPayload(transactionPayloadJson []byte,
	triggerObjectMap map[string]interface{}, t *testing.T, table models.Table,
) models.PayloadReader {
	if err := json.Unmarshal(transactionPayloadJson, &triggerObjectMap); err != nil {
		t.Fatalf("Could not unmarshal json: %s", err)
	}
	transactionPayload, err := payload_parser.ParseToDataModelObject(table, transactionPayloadJson)
	assert.NoError(t, err, "Could not parse payload")
	return transactionPayload
}

func createDecisions(t *testing.T, table models.Table, usecasesWithCreds usecases.UsecasesWithCreds, organizationId, scenarioId string, logger *slog.Logger) {
	decisionUsecase := usecasesWithCreds.NewDecisionUsecase()

	// Create a decision [REJECT]
	transactionPayloadJson := []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_reject}",
		"amount": 100
	}`)
	rejectDecision := createAndTestDecision(t, transactionPayloadJson, table, decisionUsecase,
		usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Reject, rejectDecision.Outcome,
		"Expected decision to be Reject, got %s", rejectDecision.Outcome)

	// Create a decision [APPROVE]
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}",
		"amount": 100
	}`)
	approveDecision := createAndTestDecision(t, transactionPayloadJson, table, decisionUsecase,
		usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Approve, approveDecision.Outcome,
		"Expected decision to be Approve, got %s", approveDecision.Outcome)

	// Create a decision [APPROVE] with a null field value (null field read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve_no_name}",
		"amount": 100
	}`)
	approveNoNameDecision := createAndTestDecision(t, transactionPayloadJson, table,
		decisionUsecase, usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Approve, approveNoNameDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoNameDecision.Outcome)
	if assert.NotEmpty(t, approveNoNameDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveNoNameDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, models.ErrNullFieldRead,
			"Expected error to be A field read in a rule is null, got %s", ruleExecution.Error)
	}

	// Create a decision [APPROVE] without a record in db (no row read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve_no_record}",
		"amount": 100
	}`)
	approveNoRecordDecision := createAndTestDecision(t, transactionPayloadJson, table,
		decisionUsecase, usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Approve, approveNoRecordDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveNoRecordDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveNoRecordDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, models.ErrNoRowsRead,
			"Expected error to be No rows were read from db in a rule, got %s", ruleExecution.Error)
	}

	// Create a decision [APPROVE] without a field in payload (null field read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}"
	}`)
	approveMissingFieldInPayloadDecision := createAndTestDecision(t, transactionPayloadJson,
		table, decisionUsecase, usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Approve, approveMissingFieldInPayloadDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveMissingFieldInPayloadDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveMissingFieldInPayloadDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, models.ErrNullFieldRead,
			"Expected error to be A field read in a rule is null, got %s", ruleExecution.Error)
	}

	// Create a decision [APPROVE] with a division by zero
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}",
		"amount": 0
	}`)
	approveDivisionByZeroDecision := createAndTestDecision(t, transactionPayloadJson, table,
		decisionUsecase, usecasesWithCreds, organizationId, scenarioId, logger)
	assert.Equal(t, models.Approve, approveDivisionByZeroDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveDivisionByZeroDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveDivisionByZeroDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, models.ErrDivisionByZero,
			"Expected error to be A division by zero occurred in a rule, got %s", ruleExecution.Error)
	}
}

func createTransactionPayloadAndClientObject(transactionPayloadJson []byte, t *testing.T,
	table models.Table,
) (models.PayloadReader, models.ClientObject) {
	triggerObjectMap := make(map[string]interface{})
	ClientObject := models.ClientObject{TableName: table.Name, Data: triggerObjectMap}
	transactionPayload := createTransactionPayload(transactionPayloadJson, triggerObjectMap, t, table)

	return transactionPayload, ClientObject
}

func createAndTestDecision(t *testing.T, transactionPayloadJson []byte, table models.Table,
	decisionUsecase usecases.DecisionUsecase, usecasesWithCreds usecases.UsecasesWithCreds,
	organizationId, scenarioId string, logger *slog.Logger,
) models.Decision {
	transactionPayload, ClientObject := createTransactionPayloadAndClientObject(transactionPayloadJson, t, table)

	decision, err := decisionUsecase.CreateDecision(usecasesWithCreds.Context, models.CreateDecisionInput{
		ScenarioId:              scenarioId,
		ClientObject:            ClientObject,
		OrganizationId:          organizationId,
		PayloadStructWithReader: transactionPayload,
	}, logger)
	assert.NoError(t, err, "Could not create decision")
	fmt.Println("Created decision", decision.DecisionId)

	return decision
}

func findRuleExecutionByName(ruleExecutions []models.RuleExecution, name string) models.RuleExecution {
	index := slices.IndexFunc(ruleExecutions, func(re models.RuleExecution) bool { return re.Rule.Name == name })
	return ruleExecutions[index]
}
