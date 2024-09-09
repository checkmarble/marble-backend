package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/guregu/null/v5"
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
		- sets up a pivot for the transactions table (defined as the tx's account_id)
		- creates an inbox to use in the case manager
		- sets up a workflow to send all decisions with the same pivot value to the same case
		- Creates a scenario on the organization
		- Creates a scenario iteration for the scenario, updates its body and publishes it
		- Ingests two accounts to be used in the scenario: one for a transaction to be rejected, one for a transaction to be approved
		- Creates several decision for transactions, with different cases tested
		- Snoozes a rule
		- Creates decisions again to test the snooze
		This corresponds to the critical path of our users.

		It does so by using the usecases (with credentials where applicable) and the repositories with a real local migrated docker db.
		It does not test the API layer.
	*/
	ctx := context.Background()

	// Initialize a logger and store it in the context
	ctx = utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	// Setup an organization and user credentials
	userCreds, dataModel, inboxId := setupOrgAndCreds(ctx, t, "test org with api usage")
	organizationId := userCreds.OrganizationId

	// Now that we have a user and credentials, create a container for usecases with these credentials
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, userCreds)

	rules := getRulesForFullApiTest()
	// Scenario setup
	scenarioId, _ := setupScenarioAndPublish(ctx, t, usecasesWithCreds, organizationId, inboxId, rules)

	apiCreds := setupApiCreds(ctx, t, usecasesWithCreds, organizationId)
	usecasesWithApiCreds := generateUsecaseWithCreds(testUsecases, apiCreds)

	// Ingest two accounts (parent of a transaction) to execute a full scenario: one to be rejected, one to be approved
	ingestAccounts(ctx, t, usecasesWithApiCreds, "accounts", organizationId)

	// Create a pair of decision and check that the outcome matches the expectation
	createDecisions(ctx, t, usecasesWithApiCreds, usecasesWithCreds,
		dataModel.Tables["transactions"], organizationId, scenarioId)
}

func setupApiCreds(ctx context.Context, t *testing.T, usecasesWithCreds usecases.UsecasesWithCreds, organizationId string) models.Credentials {
	// Create an API Key for this org
	apiKeyUsecase := usecasesWithCreds.NewApiKeyUseCase()
	apiKey, err := apiKeyUsecase.CreateApiKey(ctx, models.CreateApiKeyInput{
		OrganizationId: organizationId,
		Description:    "Test API key",
		Role:           models.API_CLIENT,
	})
	if err != nil {
		assert.FailNow(t, "Could not create api key", err)
	}

	_, _, creds, err := tokenGenerator.FromAPIKey(ctx, apiKey.Key)
	if err != nil {
		assert.FailNow(t, "Could not generate creds from api key", err)
	}
	return creds
}

func setupOrgAndCreds(ctx context.Context, t *testing.T, orgName string) (models.Credentials, models.DataModel, string) {
	// Create a new organization
	testAdminUsecase := generateUsecaseWithCredForMarbleAdmin(testUsecases, "")
	orgUsecase := testAdminUsecase.NewOrganizationUseCase()
	organization, err := orgUsecase.CreateOrganization(ctx, orgName)
	if err != nil {
		assert.FailNow(t, "Could not create organization", err)
	}
	organizationId := organization.Id
	fmt.Println("Created organization", organizationId)

	testAdminUsecase = generateUsecaseWithCredForMarbleAdmin(testUsecases, organizationId)

	// Check that there are no users on the organization yet
	users, err := orgUsecase.GetUsersOfOrganization(ctx, organizationId)
	if err != nil {
		assert.FailNow(t, "Could not get users of organization", err)
	}
	assert.Equal(t, 0, len(users), "Expected 0 users, got %d", len(users))

	// Create a new admin user on the organization
	userUsecase := testAdminUsecase.NewUserUseCase()
	adminUser, err := userUsecase.AddUser(ctx, models.CreateUser{
		Email:          uuid.NewString() + "@testmarble.com",
		OrganizationId: organizationId,
		Role:           models.ADMIN,
	})
	if err != nil {
		assert.FailNow(t, "Could not create user", err)
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
	usecases := generateUsecaseWithCreds(testUsecases, creds)

	// Create a data model for the organization
	dataModel, inboxId := createDataModelAndSetupCaseManager(ctx, t, usecases, organizationId)
	fmt.Println("Created data model")

	return creds, dataModel, inboxId
}

func createDataModelAndSetupCaseManager(
	ctx context.Context,
	t *testing.T,
	usecases usecases.UsecasesWithCreds,
	organizationId string,
) (dm models.DataModel, inboxId string) {
	testAdminUsecase := generateUsecaseWithCredForMarbleAdmin(testUsecases, organizationId)

	usecase := testAdminUsecase.NewDataModelUseCase()
	transactionsTableId, err := usecase.CreateDataModelTable(ctx, organizationId, "transactions", "description")
	if err != nil {
		assert.FailNow(t, "Could not create table", err)
	}
	transactionsFields := []models.CreateFieldInput{
		{TableId: transactionsTableId, Name: "account_id", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "bic_country", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "country", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "description", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "direction", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "status", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "title", DataType: models.String, Nullable: true},
		{TableId: transactionsTableId, Name: "amount", DataType: models.Float, Nullable: true},
	}
	for _, field := range transactionsFields {
		_, err = usecase.CreateDataModelField(ctx, field)
		if err != nil {
			assert.FailNow(t, "Could not create field", err)
		}
	}

	accountsTableId, err := usecase.CreateDataModelTable(ctx, organizationId, "accounts", "description")
	if err != nil {
		assert.FailNow(t, "Could not create table", err)
	}
	accountsFields := []models.CreateFieldInput{
		{TableId: accountsTableId, Name: "balance", DataType: models.Float, Nullable: true},
		{TableId: accountsTableId, Name: "company_id", DataType: models.String, Nullable: true},
		{TableId: accountsTableId, Name: "name", DataType: models.String, Nullable: true},
		{TableId: accountsTableId, Name: "currency", DataType: models.String, Nullable: true},
		{TableId: accountsTableId, Name: "is_frozen", DataType: models.Bool, Nullable: true},
	}
	for _, field := range accountsFields {
		_, err = usecase.CreateDataModelField(ctx, field)
		if err != nil {
			assert.FailNow(t, "Could not create field", err)
		}
	}

	companiesTableId, err := usecase.CreateDataModelTable(ctx, organizationId, "companies", "description")
	if err != nil {
		assert.FailNow(t, "Could not create table", err)
	}
	companiesFields := []models.CreateFieldInput{
		{TableId: companiesTableId, Name: "name", DataType: models.Float, Nullable: true},
	}
	for _, field := range companiesFields {
		_, err = usecase.CreateDataModelField(ctx, field)
		if err != nil {
			assert.FailNow(t, "Could not create field", err)
		}
	}

	dm, err = usecase.GetDataModel(ctx, organizationId)
	if err != nil {
		assert.FailNow(t, "Could not get data model", err)
	}

	txToAccountLinkId, err := usecase.CreateDataModelLink(ctx, models.DataModelLinkCreateInput{
		Name:           "account",
		OrganizationID: organizationId,
		ParentTableID:  accountsTableId,
		ParentFieldID:  dm.Tables["accounts"].Fields["object_id"].ID,
		ChildTableID:   transactionsTableId,
		ChildFieldID:   dm.Tables["transactions"].Fields["account_id"].ID,
	})
	if err != nil {
		assert.FailNow(t, "Could not create data model link", err)
	}

	_, err = usecase.CreateDataModelLink(ctx, models.DataModelLinkCreateInput{
		Name:           "company",
		OrganizationID: organizationId,
		ParentTableID:  companiesTableId,
		ParentFieldID:  dm.Tables["companies"].Fields["object_id"].ID,
		ChildTableID:   accountsTableId,
		ChildFieldID:   dm.Tables["accounts"].Fields["company_id"].ID,
	})
	if err != nil {
		assert.FailNow(t, "Could not create data model link", err)
	}

	pivot, err := usecase.CreatePivot(ctx, models.CreatePivotInput{
		BaseTableId:    transactionsTableId,
		OrganizationId: organizationId,
		PathLinkIds:    []string{txToAccountLinkId},
	})
	if err != nil {
		assert.FailNow(t, "Failed to create pivot value", err)
	}
	fmt.Printf("Created pivot %s\n", pivot.Id)

	inboxUsecase := usecases.NewInboxUsecase()
	inbox, err := inboxUsecase.CreateInbox(ctx, models.CreateInboxInput{
		Name:           "test inbox",
		OrganizationId: organizationId,
	})
	if err != nil {
		assert.FailNow(t, "could not create inbox", err)
	}
	fmt.Printf("Created inbox %s successfully\n", inbox.Id)

	dm, err = usecase.GetDataModel(ctx, organizationId)
	if err != nil {
		assert.FailNow(t, "Could not get data model", err)
	}
	return dm, inbox.Id
}

func setupScenarioAndPublish(
	ctx context.Context,
	t *testing.T,
	usecasesWithCreds usecases.UsecasesWithCreds,
	organizationId, inboxId string,
	rules []models.CreateRuleInput,
) (scenarioId, scenarioIterationId string) {
	// Create a new empty scenario
	scenarioUsecase := usecasesWithCreds.NewScenarioUsecase()
	scenario, err := scenarioUsecase.CreateScenario(ctx, models.CreateScenarioInput{
		Name:              "Test scenario",
		Description:       "Test scenario description",
		TriggerObjectType: "transactions",
		OrganizationId:    organizationId,
	})
	if err != nil {
		assert.FailNow(t, "Could not create scenario", err)
	}
	scenarioId = scenario.Id
	fmt.Println("Created scenario", scenarioId)

	assert.Equal(t, scenario.OrganizationId, organizationId)

	// Now, create a scenario iteration, with a rule
	scenarioIterationUsecase := usecasesWithCreds.NewScenarioIterationUsecase()
	threshold := 20
	scenarioIteration, err := scenarioIterationUsecase.CreateScenarioIteration(
		ctx, organizationId, models.CreateScenarioIterationInput{
			ScenarioId: scenarioId,
			Body: &models.CreateScenarioIterationBody{
				Rules: rules,
				TriggerConditionAstExpression: &ast.Node{
					Function: ast.FUNC_EQUAL,
					Children: []ast.Node{{Constant: "transactions"}, {Constant: "transactions"}},
				},
				ScoreReviewThreshold:         &threshold,
				ScoreBlockAndReviewThreshold: &threshold,
				ScoreRejectThreshold:         &threshold,
				Schedule:                     "*/10 * * * *",
			},
		})
	if err != nil {
		assert.FailNow(t, "Could not create scenario iteration", err)
	}
	scenarioIterationId = scenarioIteration.Id
	fmt.Println("Created scenario iteration", scenarioIterationId)

	// Actually, modify the scenario iteration
	threshold = 30
	updatedScenarioIteration, err := scenarioIterationUsecase.UpdateScenarioIteration(
		ctx, organizationId, models.UpdateScenarioIterationInput{
			Id: scenarioIterationId,
			Body: models.UpdateScenarioIterationBody{
				ScoreRejectThreshold: &threshold,
			},
		})
	if err != nil {
		assert.FailNow(t, "Could not update scenario iteration", err)
	}

	validation, err := scenarioIterationUsecase.ValidateScenarioIteration(ctx, scenarioIterationId, nil, nil)
	if err != nil {
		assert.FailNow(t, "Could not validate scenario iteration", err)
	}

	if scenarios.ScenarioValidationToError(validation) != nil {
		assert.FailNow(t, "Scenario iteration not valid", err)
	}

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
	if err != nil {
		assert.FailNow(t, "Could not commit scenario iteration", err)
	}
	err = scenarioPublicationUsecase.StartPublicationPreparation(ctx, organizationId, scenarioIterationId)
	if err != nil {
		assert.FailNow(t, "Could not start publication preparation", err)
	}
	time.Sleep(50 * time.Millisecond)
	scenarioPublications, err := scenarioPublicationUsecase.ExecuteScenarioPublicationAction(
		ctx,
		organizationId,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: scenarioIterationId,
			PublicationAction:   models.Publish,
		},
	)
	if err != nil {
		assert.FailNow(t, "Could not publish scenario iteration", err)
	}
	assert.Equal(t, 1, len(scenarioPublications), "Expected 1 scenario publication, got %d", len(scenarioPublications))
	fmt.Println("Published scenario iteration")

	// Now get the iteration and check it has a version
	scenarioIteration, err = scenarioIterationUsecase.GetScenarioIteration(ctx, scenarioIterationId)
	if err != nil {
		assert.FailNow(t, "Could not get scenario iteration", err)
	}

	assert.NotNil(t, scenarioIteration.Version, "Expected scenario iteration to have a version")
	if assert.NotNil(t, scenarioIteration.Version) {
		assert.Equal(t, 1, *scenarioIteration.Version,
			"Expected scenario iteration to have version")
	}
	fmt.Printf("Updated scenario iteration %+v\n", scenarioIteration)

	workflowType := models.WorkflowAddToCaseIfPossible
	_, err = scenarioUsecase.UpdateScenario(ctx, models.UpdateScenarioInput{
		Id:                         scenarioId,
		DecisionToCaseOutcomes:     []models.Outcome{models.Reject, models.Review},
		DecisionToCaseInboxId:      null.StringFrom(inboxId),
		DecisionToCaseWorkflowType: &workflowType,
	})
	if err != nil {
		assert.FailNow(t, "Failed to create workflow on scenario", err)
	}

	return scenarioId, scenarioIterationId
}

func ingestAccounts(
	ctx context.Context,
	t *testing.T,
	usecases usecases.UsecasesWithCreds,
	tableName, organizationId string,
) {
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

	_, err := ingestionUsecase.IngestObjects(ctx, organizationId, tableName, accountPayloadJson1)
	if err != nil {
		assert.FailNow(t, "Could not ingest data", err)
	}
	_, err = ingestionUsecase.IngestObjects(ctx, organizationId, tableName, accountPayloadJson2)
	if err != nil {
		assert.FailNow(t, "Could not ingest data", err)
	}
	_, err = ingestionUsecase.IngestObjects(ctx, organizationId, tableName, accountPayloadJson3)
	if err != nil {
		assert.FailNow(t, "Could not ingest data", err)
	}
}

func createDecisions(
	ctx context.Context,
	t *testing.T,
	usecasesWithApiCreds usecases.UsecasesWithCreds,
	usecasesWithUserCreds usecases.UsecasesWithCreds,
	table models.Table,
	organizationId, scenarioId string,
) {
	decisionUsecase := usecasesWithApiCreds.NewDecisionUsecase()

	// Create a decision [REJECT]
	transactionPayloadJson := []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_reject}",
		"amount": 100
	}`)
	rejectDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
		organizationId, scenarioId, 111)
	assert.Equal(t, models.Reject, rejectDecision.Outcome,
		"Expected decision to be Reject, got %s", rejectDecision.Outcome)

	// Create a second decision with the same account_id to check their cases [REJECT]
	rejectDecision2 := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
		organizationId, scenarioId, 111)
	assert.Equal(t, models.Reject, rejectDecision.Outcome,
		"Expected decision to be Reject, got %s", rejectDecision.Outcome)

	// Check that the two decisions on tx with account_id "{account_id_reject}" are both in a case - the same
	assert.NotNil(t, rejectDecision.Case, "Decision is in a case")
	assert.NotNil(t, rejectDecision2.Case, "Decision is in a case")
	assert.Equal(t, rejectDecision.Case.Id, rejectDecision2.Case.Id,
		"The two decisions are in the same case")

	// Create a decision [APPROVE]
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}",
		"amount": 100
	}`)
	approveDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
		organizationId, scenarioId, 11)
	assert.Equal(t, models.Approve, approveDecision.Outcome,
		"Expected decision to be Approve, got %s", approveDecision.Outcome)
	assert.Nil(t, approveDecision.Case, "Approve decision is not in a case")

	// Create a decision [APPROVE] with a null field value (null field read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve_no_name}",
		"amount": 100
	}`)
	approveNoNameDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
		decisionUsecase, organizationId, scenarioId, 11)
	assert.Equal(t, models.Approve, approveNoNameDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoNameDecision.Outcome)
	if assert.NotEmpty(t, approveNoNameDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveNoNameDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNullFieldRead,
			"Expected error to be \"%s\", got \"%s\"", ast.ErrNullFieldRead, ruleExecution.Error)
	}

	// Create a decision [APPROVE] without a record in db (no row read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve_no_record}",
		"amount": 100
	}`)
	approveNoRecordDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
		decisionUsecase, organizationId, scenarioId, 11)
	assert.Equal(t, models.Approve, approveNoRecordDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveNoRecordDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveNoRecordDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNoRowsRead,
			"Expected error to be \"%s\", got \"%s\"", ast.ErrNoRowsRead, ruleExecution.Error)
	}

	// Create a decision [APPROVE] without a field in payload (null field read)
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}"
	}`)
	approveMissingFieldInPayloadDecision := createAndTestDecision(ctx, t, transactionPayloadJson,
		table, decisionUsecase, organizationId, scenarioId, 1)
	assert.Equal(t, models.Approve, approveMissingFieldInPayloadDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveMissingFieldInPayloadDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveMissingFieldInPayloadDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNullFieldRead,
			"Expected error to be \"%s\", got \"%s\"", ast.ErrNullFieldRead, ruleExecution.Error)
	}

	// Create a decision [APPROVE] with a division by zero
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_approve}",
		"amount": 0
	}`)
	approveDivisionByZeroDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
		decisionUsecase, organizationId, scenarioId, 11)
	assert.Equal(t, models.Approve, approveDivisionByZeroDecision.Outcome,
		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
	if assert.NotEmpty(t, approveDivisionByZeroDecision.RuleExecutions) {
		ruleExecution := findRuleExecutionByName(approveDivisionByZeroDecision.RuleExecutions, "Check on account name")
		assert.ErrorIs(t, ruleExecution.Error, ast.ErrDivisionByZero,
			"Expected error to be \"%s\", got \"%s\"", ast.ErrDivisionByZero, ruleExecution.Error)
	}

	// find the rule with higest score
	ruleId := ""
	for _, r := range rejectDecision.RuleExecutions {
		if r.Rule.Name == "Check on account name" {
			ruleId = r.Rule.Id
		}
	}
	// Now snooze the rules and rerun the decision in REJECT status
	ruleSnoozeUsecase := usecasesWithUserCreds.NewRuleSnoozeUsecase()
	_, err := ruleSnoozeUsecase.SnoozeDecision(ctx, models.SnoozeDecisionInput{
		Comment:        "this is a test snooze",
		DecisionId:     rejectDecision.DecisionId,
		Duration:       "500ms", // snooze for 0.5 sec, after this wait for the snooze to end before moving on
		OrganizationId: organizationId,
		RuleId:         ruleId, // snooze a rule (nevermind which one)
		UserId:         usecasesWithUserCreds.Credentials.ActorIdentity.UserId,
	})
	if err != nil {
		assert.FailNow(t, "Failed to snooze decision", err)
	}
	// After snoozing the rule, the new decision should be approved
	transactionPayloadJson = []byte(`{
		"object_id": "{transaction_id}",
		"updated_at": "2020-01-01T00:00:00Z",
		"account_id": "{account_id_reject}",
		"amount": 100
	}`)
	approvedDecisionAfternooze := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
		organizationId, scenarioId, 11)
	assert.Equal(t, models.Approve, approvedDecisionAfternooze.Outcome,
		"Expected decision to be Approve, got %s", approvedDecisionAfternooze.Outcome)
	time.Sleep(time.Millisecond * 500)
}

func createAndTestDecision(
	ctx context.Context,
	t *testing.T,
	transactionPayloadJson []byte,
	table models.Table,
	decisionUsecase usecases.DecisionUsecase,
	organizationId string,
	scenarioId string,
	expectedScore int,
) models.DecisionWithRuleExecutions {
	parser := payload_parser.NewParser()
	transactionPayload, validationErrors, err :=
		parser.ParsePayload(table, transactionPayloadJson)
	if err != nil {
		assert.FailNow(t, "Could not parse payload", err)
	}
	assert.Empty(t, validationErrors, "Expected no validation errors, got %v", validationErrors)

	decision, err := decisionUsecase.CreateDecision(
		ctx,
		models.CreateDecisionInput{
			ScenarioId:         scenarioId,
			ClientObject:       &transactionPayload,
			OrganizationId:     organizationId,
			TriggerObjectTable: table.Name,
		},
		false,
		false,
	)
	if err != nil {
		assert.FailNow(t, "Could not create decision", err)
	}
	assert.Equal(t, expectedScore, decision.Score, "The score should match the expected value")
	fmt.Println("Created decision", decision.DecisionId)

	return decision
}

func findRuleExecutionByName(ruleExecutions []models.RuleExecution, name string) models.RuleExecution {
	index := slices.IndexFunc(ruleExecutions, func(re models.RuleExecution) bool { return re.Rule.Name == name })
	return ruleExecutions[index]
}

func getRulesForFullApiTest() []models.CreateRuleInput {
	return []models.CreateRuleInput{
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
		{
			FormulaAstExpression: &ast.Node{
				Function: ast.FUNC_GREATER,
				Children: []ast.Node{
					{
						Function: ast.FUNC_FUZZY_MATCH,
						Children: []ast.Node{
							{Constant: "testy testing"},
							{Constant: "tasty tasting"},
						},
						NamedChildren: map[string]ast.Node{
							"algorithm": {Constant: "ratio"},
						},
					},
					{Constant: 50},
				},
			},
			ScoreModifier: 1,
			Name:          "Fuzzy match on name",
			Description:   "Fuzzy match on name",
		},
	}
}
