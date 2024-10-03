package integration

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/segmentio/analytics-go/v3"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func TestBatchIngestionAndExecution(t *testing.T) {
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
		- Validates and uploads a csv file (one line) with data of transactions
		- Runs the batch job that executes data ingestion from CSV files
		- Schedules (manually) a batch execution
		- Runs the batch job that executes all scenarios marked as due

		It does so by using the usecases (with credentials where applicable) and the repositories with a real local migrated docker db.
		It does not test the API layer.
	*/
	ctx := context.Background()

	// Initialize a logger and store it in the context
	ctx = utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	// Setup an organization and user credentials
	userCreds, _, inboxId := setupOrgAndCreds(ctx, t, "test org with batch usage")
	organizationId := userCreds.OrganizationId

	// Now that we have a user and credentials, create a container for usecases with these credentials
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, userCreds)

	rules := getRulesForBatchTest()
	// Scenario setup
	scenarioId, scenarioIterationId := setupScenarioAndPublish(ctx, t, usecasesWithCreds, organizationId, inboxId, rules)

	// Ingest two accounts (parent of a transaction) to execute a full scenario: one to be declined, one to be approved
	ingestAccountsBatch(ctx, t, usecasesWithCreds, organizationId, string(userCreds.ActorIdentity.UserId))

	// Create a pair of decision and check that the outcome matches the expectation
	createDecisionsBatch(ctx, t, usecasesWithCreds, organizationId, scenarioId, scenarioIterationId)
}

func ingestAccountsBatch(
	ctx context.Context,
	t *testing.T,
	usecases usecases.UsecasesWithCreds,
	organizationId, userId string,
) {
	ingestionUsecase := usecases.NewIngestionUseCase()
	fileContent := `object_id,updated_at,account_id,bic_country,country,description,status,title,amount
a8ca9ad7-1581-44f8-89d0-1f00500f2d02,2024-08-11T22:47:00Z,7b7ffdbc-cf98-48cb-a468-7695122d74d6,france,germany,"some tx description",validated,"some tx title",4200
`
	reader := csv.NewReader(strings.NewReader(fileContent))
	log, err := ingestionUsecase.ValidateAndUploadIngestionCsv(ctx, organizationId, userId, "transactions", reader)
	if err != nil {
		assert.FailNow(t, "failed to validate and upload ingestion csv", err)
	}
	fmt.Printf("Created upload log %s in pending state", log.Id)

	err = ingestionUsecase.IngestDataFromCsv(ctx)
	if err != nil {
		assert.FailNow(t, "Failed to ingest data from csv file", err)
	}

	logs, err := ingestionUsecase.ListUploadLogs(ctx, organizationId, "transactions")
	if err != nil {
		assert.FailNow(t, "Failed to read upload logs", err)
	}
	assert.Len(t, logs, 1, "There should be one upload log")
	assert.Equal(t, logs[0].UploadStatus, models.UploadStatus("success"),
		"The upload log should be in success state")
	assert.Equal(t, logs[0].LinesProcessed, 1, "The upload log should have processed 1 line")
}

func createDecisionsBatch(
	ctx context.Context,
	t *testing.T,
	usecasesWithUserCreds usecases.UsecasesWithCreds,
	organizationId, scenarioId, scenarioIterationId string,
) {
	scheduledExecUsecase := usecasesWithUserCreds.NewScheduledExecutionUsecase()
	ses, err := scheduledExecUsecase.ListScheduledExecutions(ctx, organizationId, &scenarioId)
	if err != nil {
		assert.FailNow(t, "Failed to list scheduled executions", err)
	}
	if len(ses) != 0 {
		assert.FailNowf(t, "wrong number of scheduled executions",
			"Expected zero scheduled execution for the scenario %s, got %d", scenarioId, len(ses))
	}

	err = scheduledExecUsecase.CreateScheduledExecution(ctx, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: scenarioIterationId,
		Manual:              true,
	})
	if err != nil {
		assert.FailNow(t, "Failed to create scheduled executions", err)
	}

	ses, err = scheduledExecUsecase.ListScheduledExecutions(ctx, organizationId, &scenarioId)
	if err != nil {
		assert.FailNow(t, "Failed to list scheduled executions", err)
	}
	if len(ses) != 1 {
		assert.FailNowf(t, "wrong number of scheduled executions",
			"Expected one scheduled execution for the scenario %s, got %d", scenarioId, len(ses))
	}

	runScheduledExecUsecase := usecasesWithUserCreds.NewRunScheduledExecution()
	err = runScheduledExecUsecase.ExecuteAllScheduledScenarios(ctx)
	if err != nil {
		assert.FailNow(t, "Failed to run scheduled executions", err)
	}

	se, err := scheduledExecUsecase.GetScheduledExecution(ctx, ses[0].Id)
	if err != nil {
		assert.FailNow(t, "Failed to get scheduled execution", err)
	}
	assert.Equal(t, models.ScheduledExecutionSuccess, se.Status, "Status should be success")
	assert.NotNil(t, se.NumberOfCreatedDecisions, "Should have created decisions")
	assert.NotNil(t, se.NumberOfEvaluatedDecisions, "Should have evaluated decisions")
	assert.NotNil(t, se.NumberOfPlannedDecisions, "Should have planned decisions")
	assert.Equal(t, 1, *se.NumberOfCreatedDecisions, "Should have created 1 decision")
	assert.Equal(t, 1, *se.NumberOfEvaluatedDecisions, "Should have evaluated 1 decision")
	assert.Equal(t, 1, *se.NumberOfPlannedDecisions, "Should have planned 1 decision")

	decisionsUsecase := usecasesWithUserCreds.NewDecisionUsecase()
	decisions, err := decisionsUsecase.ListDecisions(ctx, organizationId,
		models.NewDefaultPaginationAndSorting("created_at"), dto.DecisionFilters{
			ScheduledExecutionIds: []string{se.Id},
		},
	)
	if err != nil {
		assert.FailNow(t, "Error while listing decisions", err)
	}
	assert.Equalf(t, 1, len(decisions), "Expected 1 decision, got %d", len(decisions))
	assert.Equalf(t, models.Decline, decisions[0].Outcome,
		"Decision should be in review status, got %s", decisions[0].Outcome)
}

func getRulesForBatchTest() []models.CreateRuleInput {
	return []models.CreateRuleInput{
		{
			FormulaAstExpression: &ast.Node{
				Function: ast.FUNC_EQUAL,
				Children: []ast.Node{
					{Constant: 1},
					{Constant: 1},
				},
			},
			ScoreModifier: 100,
			Name:          "Rule that hits",
			Description:   "Rule that hits",
		},
	}
}
