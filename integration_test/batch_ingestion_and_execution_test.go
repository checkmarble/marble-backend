package integration

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/segmentio/analytics-go/v3"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
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
		- ...

		It does so by using the usecases (with credentials where applicable) and the repositories with a real local migrated docker db.
		It does not test the API layer.
	*/
	ctx := context.Background()

	// Initialize a logger and store it in the context
	ctx = utils.StoreLoggerInContext(ctx, utils.NewLogger("text"))
	ctx = utils.StoreSegmentClientInContext(ctx, analytics.New("dummy key"))

	// Setup an organization and user credentials
	userCreds, _, inboxId := setupOrgAndCreds(ctx, t)
	organizationId := userCreds.OrganizationId

	// Now that we have a user and credentials, create a container for usecases with these credentials
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, userCreds)

	// Scenario setup
	_ = setupScenarioAndPublish(ctx, t, usecasesWithCreds, organizationId, inboxId)

	apiCreds := setupApiCreds(ctx, t, usecasesWithCreds, organizationId)
	usecasesWithApiCreds := generateUsecaseWithCreds(testUsecases, apiCreds)

	// Ingest two accounts (parent of a transaction) to execute a full scenario: one to be rejected, one to be approved
	ingestAccountsBatch(ctx, t, usecasesWithApiCreds, organizationId, string(userCreds.ActorIdentity.UserId))

	// Create a pair of decision and check that the outcome matches the expectation
	// createDecisionsBatch(ctx, t, usecasesWithApiCreds, usecasesWithCreds,
	// 	dataModel.Tables["transactions"], organizationId, scenarioId)
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

	err = ingestionUsecase.IngestDataFromCsv(ctx, utils.LoggerFromContext(ctx))
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

// func createDecisionsBatch(
// 	ctx context.Context,
// 	t *testing.T,
// 	usecasesWithApiCreds usecases.UsecasesWithCreds,
// 	usecasesWithUserCreds usecases.UsecasesWithCreds,
// 	table models.Table,
// 	organizationId, scenarioId string,
// ) {
// 	decisionUsecase := usecasesWithApiCreds.NewDecisionUsecase()

// 	// Create a decision [REJECT]
// 	transactionPayloadJson := []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_reject}",
// 		"amount": 100
// 	}`)
// 	rejectDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
// 		organizationId, scenarioId, 111)
// 	assert.Equal(t, models.Reject, rejectDecision.Outcome,
// 		"Expected decision to be Reject, got %s", rejectDecision.Outcome)

// 	// Create a second decision with the same account_id to check their cases [REJECT]
// 	rejectDecision2 := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
// 		organizationId, scenarioId, 111)
// 	assert.Equal(t, models.Reject, rejectDecision.Outcome,
// 		"Expected decision to be Reject, got %s", rejectDecision.Outcome)

// 	// Check that the two decisions on tx with account_id "{account_id_reject}" are both in a case - the same
// 	assert.NotNil(t, rejectDecision.Case, "Decision is in a case")
// 	assert.NotNil(t, rejectDecision2.Case, "Decision is in a case")
// 	assert.Equal(t, rejectDecision.Case.Id, rejectDecision2.Case.Id,
// 		"The two decisions are in the same case")

// 	// Create a decision [APPROVE]
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_approve}",
// 		"amount": 100
// 	}`)
// 	approveDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
// 		organizationId, scenarioId, 11)
// 	assert.Equal(t, models.Approve, approveDecision.Outcome,
// 		"Expected decision to be Approve, got %s", approveDecision.Outcome)
// 	assert.Nil(t, approveDecision.Case, "Approve decision is not in a case")

// 	// Create a decision [APPROVE] with a null field value (null field read)
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_approve_no_name}",
// 		"amount": 100
// 	}`)
// 	approveNoNameDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
// 		decisionUsecase, organizationId, scenarioId, 11)
// 	assert.Equal(t, models.Approve, approveNoNameDecision.Outcome,
// 		"Expected decision to be Approve, got %s", approveNoNameDecision.Outcome)
// 	if assert.NotEmpty(t, approveNoNameDecision.RuleExecutions) {
// 		ruleExecution := findRuleExecutionByName(approveNoNameDecision.RuleExecutions, "Check on account name")
// 		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNullFieldRead,
// 			"Expected error to be \"%s\", got \"%s\"", ast.ErrNullFieldRead, ruleExecution.Error)
// 	}

// 	// Create a decision [APPROVE] without a record in db (no row read)
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_approve_no_record}",
// 		"amount": 100
// 	}`)
// 	approveNoRecordDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
// 		decisionUsecase, organizationId, scenarioId, 11)
// 	assert.Equal(t, models.Approve, approveNoRecordDecision.Outcome,
// 		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
// 	if assert.NotEmpty(t, approveNoRecordDecision.RuleExecutions) {
// 		ruleExecution := findRuleExecutionByName(approveNoRecordDecision.RuleExecutions, "Check on account name")
// 		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNoRowsRead,
// 			"Expected error to be \"%s\", got \"%s\"", ast.ErrNoRowsRead, ruleExecution.Error)
// 	}

// 	// Create a decision [APPROVE] without a field in payload (null field read)
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_approve}"
// 	}`)
// 	approveMissingFieldInPayloadDecision := createAndTestDecision(ctx, t, transactionPayloadJson,
// 		table, decisionUsecase, organizationId, scenarioId, 1)
// 	assert.Equal(t, models.Approve, approveMissingFieldInPayloadDecision.Outcome,
// 		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
// 	if assert.NotEmpty(t, approveMissingFieldInPayloadDecision.RuleExecutions) {
// 		ruleExecution := findRuleExecutionByName(approveMissingFieldInPayloadDecision.RuleExecutions, "Check on account name")
// 		assert.ErrorIs(t, ruleExecution.Error, ast.ErrNullFieldRead,
// 			"Expected error to be \"%s\", got \"%s\"", ast.ErrNullFieldRead, ruleExecution.Error)
// 	}

// 	// Create a decision [APPROVE] with a division by zero
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_approve}",
// 		"amount": 0
// 	}`)
// 	approveDivisionByZeroDecision := createAndTestDecision(ctx, t, transactionPayloadJson, table,
// 		decisionUsecase, organizationId, scenarioId, 11)
// 	assert.Equal(t, models.Approve, approveDivisionByZeroDecision.Outcome,
// 		"Expected decision to be Approve, got %s", approveNoRecordDecision.Outcome)
// 	if assert.NotEmpty(t, approveDivisionByZeroDecision.RuleExecutions) {
// 		ruleExecution := findRuleExecutionByName(approveDivisionByZeroDecision.RuleExecutions, "Check on account name")
// 		assert.ErrorIs(t, ruleExecution.Error, ast.ErrDivisionByZero,
// 			"Expected error to be \"%s\", got \"%s\"", ast.ErrDivisionByZero, ruleExecution.Error)
// 	}

// 	// find the rule with higest score
// 	ruleId := ""
// 	for _, r := range rejectDecision.RuleExecutions {
// 		if r.Rule.Name == "Check on account name" {
// 			ruleId = r.Rule.Id
// 		}
// 	}
// 	// Now snooze the rules and rerun the decision in REJECT status
// 	ruleSnoozeUsecase := usecasesWithUserCreds.NewRuleSnoozeUsecase()
// 	_, err := ruleSnoozeUsecase.SnoozeDecision(ctx, models.SnoozeDecisionInput{
// 		Comment:        "this is a test snooze",
// 		DecisionId:     rejectDecision.DecisionId,
// 		Duration:       "500ms", // snooze for 0.5 sec, after this wait for the snooze to end before moving on
// 		OrganizationId: organizationId,
// 		RuleId:         ruleId, // snooze a rule (nevermind which one)
// 		UserId:         usecasesWithUserCreds.Credentials.ActorIdentity.UserId,
// 	})
// 	if err != nil {
// 		assert.FailNow(t, "Failed to snooze decision", err)
// 	}
// 	// After snoozing the rule, the new decision should be approved
// 	transactionPayloadJson = []byte(`{
// 		"object_id": "{transaction_id}",
// 		"updated_at": "2020-01-01T00:00:00Z",
// 		"account_id": "{account_id_reject}",
// 		"amount": 100
// 	}`)
// 	approvedDecisionAfternooze := createAndTestDecision(ctx, t, transactionPayloadJson, table, decisionUsecase,
// 		organizationId, scenarioId, 11)
// 	assert.Equal(t, models.Approve, approvedDecisionAfternooze.Outcome,
// 		"Expected decision to be Approve, got %s", approvedDecisionAfternooze.Outcome)
// 	time.Sleep(time.Millisecond * 500)
// }
