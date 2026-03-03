package worker_jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// --- Mock implementations for unexported interfaces ---

type mockAsyncDecisionIngester struct {
	mock.Mock
}

func (m *mockAsyncDecisionIngester) IngestObject(
	ctx context.Context,
	organizationId uuid.UUID,
	objectType string,
	objectBody json.RawMessage,
	ingestionOptions models.IngestionOptions,
	parserOpts ...payload_parser.ParserOpt,
) (int, error) {
	args := m.Called(ctx, organizationId, objectType, objectBody, ingestionOptions)
	return args.Int(0), args.Error(1)
}

type mockAsyncDecisionCreator struct {
	mock.Mock
}

func (m *mockAsyncDecisionCreator) CreateAllDecisions(
	ctx context.Context,
	input models.CreateAllDecisionsInput,
	params models.CreateDecisionParams,
) ([]models.DecisionWithRuleExecutions, int, error) {
	args := m.Called(ctx, input, params)
	return args.Get(0).([]models.DecisionWithRuleExecutions), args.Int(1), args.Error(2)
}

type mockAsyncDecisionExecutionRepo struct {
	mock.Mock
}

func (m *mockAsyncDecisionExecutionRepo) GetAsyncDecisionExecution(
	ctx context.Context,
	exec repositories.Executor,
	id uuid.UUID,
) (models.AsyncDecisionExecution, error) {
	args := m.Called(ctx, exec, id)
	return args.Get(0).(models.AsyncDecisionExecution), args.Error(1)
}

func (m *mockAsyncDecisionExecutionRepo) UpdateAsyncDecisionExecution(
	ctx context.Context,
	exec repositories.Executor,
	input models.AsyncDecisionExecutionUpdate,
) error {
	args := m.Called(ctx, exec, input)
	return args.Error(0)
}

type mockWebhookEventsUsecase struct {
	mock.Mock
}

func (m *mockWebhookEventsUsecase) CreateWebhookEvent(
	ctx context.Context,
	tx repositories.Transaction,
	input models.WebhookEventCreate,
) error {
	args := m.Called(ctx, tx, input)
	return args.Error(0)
}

func (m *mockWebhookEventsUsecase) SendWebhookEventAsync(ctx context.Context, webhookEventId string) {
	m.Called(ctx, webhookEventId)
}

// --- Test suite ---

type AsyncDecisionExecutionWorkerTestSuite struct {
	suite.Suite
	executionRepo      *mockAsyncDecisionExecutionRepo
	ingester           *mockAsyncDecisionIngester
	decisionCreator    *mockAsyncDecisionCreator
	webhookSender      *mockWebhookEventsUsecase
	executorFactory    executor_factory.ExecutorFactoryStub
	transactionFactory executor_factory.TransactionFactoryStub

	ctx   context.Context
	orgId uuid.UUID
}

func (s *AsyncDecisionExecutionWorkerTestSuite) SetupTest() {
	s.executionRepo = new(mockAsyncDecisionExecutionRepo)
	s.ingester = new(mockAsyncDecisionIngester)
	s.decisionCreator = new(mockAsyncDecisionCreator)
	s.webhookSender = new(mockWebhookEventsUsecase)

	s.executorFactory = executor_factory.NewExecutorFactoryStub()
	s.transactionFactory = executor_factory.NewTransactionFactoryStub(s.executorFactory)

	s.ctx = utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	s.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
}

func (s *AsyncDecisionExecutionWorkerTestSuite) makeWorker() *AsyncDecisionExecutionWorker {
	return NewAsyncDecisionExecutionWorker(
		s.executionRepo,
		s.executorFactory,
		s.transactionFactory,
		s.ingester,
		s.decisionCreator,
		s.webhookSender,
	)
}

func (s *AsyncDecisionExecutionWorkerTestSuite) makeJob(executionId uuid.UUID) *river.Job[models.AsyncDecisionExecutionArgs] {
	return &river.Job[models.AsyncDecisionExecutionArgs]{
		JobRow: &rivertype.JobRow{
			Attempt:     1,
			MaxAttempts: 5,
		},
		Args: models.AsyncDecisionExecutionArgs{
			AsyncDecisionExecutionId: executionId.String(),
		},
	}
}

func (s *AsyncDecisionExecutionWorkerTestSuite) AssertExpectations() {
	t := s.T()
	s.executionRepo.AssertExpectations(t)
	s.ingester.AssertExpectations(t)
	s.decisionCreator.AssertExpectations(t)
	s.webhookSender.AssertExpectations(t)
}

func TestAsyncDecisionExecutionWorker(t *testing.T) {
	suite.Run(t, new(AsyncDecisionExecutionWorkerTestSuite))
}

// Test 1: Completed status is a no-op
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_CompletedStatus_Noop() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(models.AsyncDecisionExecution{
			Id:     executionId,
			OrgId:  s.orgId,
			Status: models.AsyncDecisionExecutionStatusCompleted,
		}, nil)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	assert.NoError(s.T(), err)
	s.ingester.AssertNotCalled(s.T(), "IngestObject")
	s.decisionCreator.AssertNotCalled(s.T(), "CreateAllDecisions")
	s.AssertExpectations()
}

// Test 2: Failed status is a no-op
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_FailedStatus_Noop() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(models.AsyncDecisionExecution{
			Id:     executionId,
			OrgId:  s.orgId,
			Status: models.AsyncDecisionExecutionStatusFailed,
		}, nil)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	assert.NoError(s.T(), err)
	s.ingester.AssertNotCalled(s.T(), "IngestObject")
	s.decisionCreator.AssertNotCalled(s.T(), "CreateAllDecisions")
	s.AssertExpectations()
}

// Test 3: Pending with ShouldIngest → ingests then creates decisions
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_PendingWithIngestion_IngestsThenCreatesDecisions() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)
	decisionId := uuid.Must(uuid.NewV7())

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  true,
		Status:        models.AsyncDecisionExecutionStatusPending,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Ingestion succeeds
	s.ingester.On("IngestObject", mock.Anything, s.orgId, "transactions", triggerObject, models.IngestionOptions{}).
		Return(1, nil)

	// Checkpoint: update status to ingested
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		models.AsyncDecisionExecutionUpdate{
			Id:     executionId,
			Status: utils.Ptr(models.AsyncDecisionExecutionStatusIngested),
		}).Return(nil)

	// Decision creation succeeds
	decisions := []models.DecisionWithRuleExecutions{
		{Decision: models.Decision{DecisionId: decisionId}},
	}
	s.decisionCreator.On("CreateAllDecisions", mock.Anything,
		models.CreateAllDecisionsInput{
			OrganizationId:     s.orgId,
			TriggerObjectTable: "transactions",
			PayloadRaw:         triggerObject,
		},
		models.CreateDecisionParams{
			WithDecisionWebhooks:        true,
			WithRuleExecutionDetails:    true,
			WithScenarioPermissionCheck: false,
			WithDisallowUnknownFields:   false,
		}).Return(decisions, 1, nil)

	// Mark completed
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		models.AsyncDecisionExecutionUpdate{
			Id:          executionId,
			Status:      utils.Ptr(models.AsyncDecisionExecutionStatusCompleted),
			DecisionIds: &[]uuid.UUID{decisionId},
		}).Return(nil)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	assert.NoError(s.T(), err)
	s.AssertExpectations()
}

// Test 4: Ingested status → skips ingestion, creates decisions
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_IngestedStatus_SkipsIngestion() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)
	decisionId := uuid.Must(uuid.NewV7())

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  true,
		Status:        models.AsyncDecisionExecutionStatusIngested,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Decision creation succeeds
	decisions := []models.DecisionWithRuleExecutions{
		{Decision: models.Decision{DecisionId: decisionId}},
	}
	s.decisionCreator.On("CreateAllDecisions", mock.Anything,
		models.CreateAllDecisionsInput{
			OrganizationId:     s.orgId,
			TriggerObjectTable: "transactions",
			PayloadRaw:         triggerObject,
		},
		models.CreateDecisionParams{
			WithDecisionWebhooks:        true,
			WithRuleExecutionDetails:    true,
			WithScenarioPermissionCheck: false,
			WithDisallowUnknownFields:   false,
		}).Return(decisions, 1, nil)

	// Mark completed
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		models.AsyncDecisionExecutionUpdate{
			Id:          executionId,
			Status:      utils.Ptr(models.AsyncDecisionExecutionStatusCompleted),
			DecisionIds: &[]uuid.UUID{decisionId},
		}).Return(nil)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	assert.NoError(s.T(), err)
	// Verify ingestion was NOT called
	s.ingester.AssertNotCalled(s.T(), "IngestObject")
	s.AssertExpectations()
}

// Test 5: Non-retryable error (NotFoundError) → marks failed and sends webhook
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_NonRetryableError_MarksFailed() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  false,
		Status:        models.AsyncDecisionExecutionStatusPending,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Decision creation fails with NotFoundError
	s.decisionCreator.On("CreateAllDecisions", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.DecisionWithRuleExecutions(nil), 0, models.NotFoundError)

	// Expect update to failed
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.AsyncDecisionExecutionUpdate) bool {
			return input.Id == executionId &&
				input.Status != nil && *input.Status == models.AsyncDecisionExecutionStatusFailed &&
				input.ErrorMessage != nil
		})).Return(nil)

	// Expect webhook creation
	s.webhookSender.On("CreateWebhookEvent", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.WebhookEventCreate) bool {
			return input.OrganizationId == s.orgId
		})).Return(nil)

	// Expect async webhook send
	s.webhookSender.On("SendWebhookEventAsync", mock.Anything, mock.AnythingOfType("string")).Return()

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	// Should return nil because the failure was handled
	assert.NoError(s.T(), err)
	s.AssertExpectations()
}

// Test 6: Last attempt with retryable error → marks failed
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_LastAttempt_MarksFailed() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	// Set to last attempt
	job.JobRow.Attempt = 5
	job.JobRow.MaxAttempts = 5
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  false,
		Status:        models.AsyncDecisionExecutionStatusPending,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Decision creation fails with a generic (retryable) error
	s.decisionCreator.On("CreateAllDecisions", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.DecisionWithRuleExecutions(nil), 0, assert.AnError)

	// Expect update to failed
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.AsyncDecisionExecutionUpdate) bool {
			return input.Id == executionId &&
				input.Status != nil && *input.Status == models.AsyncDecisionExecutionStatusFailed &&
				input.ErrorMessage != nil
		})).Return(nil)

	// Expect webhook creation
	s.webhookSender.On("CreateWebhookEvent", mock.Anything, mock.Anything,
		mock.MatchedBy(func(input models.WebhookEventCreate) bool {
			return input.OrganizationId == s.orgId
		})).Return(nil)

	// Expect async webhook send
	s.webhookSender.On("SendWebhookEventAsync", mock.Anything, mock.AnythingOfType("string")).Return()

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	// Should return nil because failure was handled (last attempt)
	assert.NoError(s.T(), err)
	s.AssertExpectations()
}

// Test 7: Retryable error on non-last attempt → returns error for River to retry
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_RetryableError_ReturnsError() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	// Not last attempt
	job.JobRow.Attempt = 2
	job.JobRow.MaxAttempts = 5
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  false,
		Status:        models.AsyncDecisionExecutionStatusPending,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Decision creation fails with a generic (retryable) error
	s.decisionCreator.On("CreateAllDecisions", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.DecisionWithRuleExecutions(nil), 0, assert.AnError)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	// Should return error so River can retry
	assert.Error(s.T(), err)
	// Should NOT have called update or webhook since it's a retry
	s.executionRepo.AssertNotCalled(s.T(), "UpdateAsyncDecisionExecution",
		s.ctx, mock.Anything, mock.MatchedBy(func(input models.AsyncDecisionExecutionUpdate) bool {
			return input.Status != nil && *input.Status == models.AsyncDecisionExecutionStatusFailed
		}))
	s.webhookSender.AssertNotCalled(s.T(), "CreateWebhookEvent")
	s.webhookSender.AssertNotCalled(s.T(), "SendWebhookEventAsync")
	s.AssertExpectations()
}

// Test 8: Pending without ShouldIngest → skips ingestion, creates decisions
func (s *AsyncDecisionExecutionWorkerTestSuite) TestWork_PendingNoIngestion_SkipsIngestion() {
	executionId := uuid.Must(uuid.NewV7())
	job := s.makeJob(executionId)
	triggerObject := json.RawMessage(`{"object_id": "obj-1",'updated_at":"2000-01-01T00:00:00Z"}`)
	decisionId := uuid.Must(uuid.NewV7())

	execution := models.AsyncDecisionExecution{
		Id:            executionId,
		OrgId:         s.orgId,
		ObjectType:    "transactions",
		TriggerObject: triggerObject,
		ShouldIngest:  false,
		Status:        models.AsyncDecisionExecutionStatusPending,
	}

	s.executionRepo.On("GetAsyncDecisionExecution", mock.Anything, mock.Anything, executionId).
		Return(execution, nil)

	// Decision creation succeeds
	decisions := []models.DecisionWithRuleExecutions{
		{Decision: models.Decision{DecisionId: decisionId}},
	}
	s.decisionCreator.On("CreateAllDecisions", mock.Anything, mock.Anything, mock.Anything).
		Return(decisions, 1, nil)

	// Mark completed
	s.executionRepo.On("UpdateAsyncDecisionExecution", mock.Anything, mock.Anything,
		models.AsyncDecisionExecutionUpdate{
			Id:          executionId,
			Status:      utils.Ptr(models.AsyncDecisionExecutionStatusCompleted),
			DecisionIds: &[]uuid.UUID{decisionId},
		}).Return(nil)

	worker := s.makeWorker()
	err := worker.Work(s.ctx, job)

	assert.NoError(s.T(), err)
	s.ingester.AssertNotCalled(s.T(), "IngestObject")
	s.AssertExpectations()
}
