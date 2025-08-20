package ai_agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

// mockWriteCloser is a mock implementation of io.WriteCloser for testing blob writes
type mockWriteCloser struct {
	*bytes.Buffer
	closed bool
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

func newMockWriteCloser() *mockWriteCloser {
	return &mockWriteCloser{
		Buffer: bytes.NewBuffer(nil),
		closed: false,
	}
}

// mockReadCloser is a mock implementation of io.ReadCloser for testing blob reads
type mockReadCloser struct {
	*strings.Reader
	closed bool
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func newMockReadCloser(data string) *mockReadCloser {
	return &mockReadCloser{
		Reader: strings.NewReader(data),
		closed: false,
	}
}

// Local mocks to avoid import cycles
// We can't use the global mocks from the mocks/ package because importing them
// would create a cycle: usecases/ai_agent -> mocks -> usecases/ai_agent
// (since mocks need to import ai_agent.CaseReviewContext)

type mockCaseReviewUsecase struct {
	mock.Mock
}

func (r *mockCaseReviewUsecase) CreateCaseReviewSync(ctx context.Context, caseId string,
	caseReviewContext *CaseReviewContext,
) (agent_dto.AiCaseReviewDto, error) {
	args := r.Called(ctx, caseId, caseReviewContext)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(agent_dto.AiCaseReviewDto), args.Error(1)
}

func (r *mockCaseReviewUsecase) HasAiCaseReviewEnabled(ctx context.Context, orgId string) (bool, error) {
	args := r.Called(ctx, orgId)
	return args.Bool(0), args.Error(1)
}

type CaseReviewWorkerTestSuite struct {
	suite.Suite
	exec              *mocks.Executor
	executorFactory   *mocks.ExecutorFactory
	blobRepo          *mocks.MockBlobRepository
	caseReviewUsecase *mockCaseReviewUsecase
	workerRepo        *mocks.MockCaseReviewWorkerRepository

	ctx context.Context
}

func (suite *CaseReviewWorkerTestSuite) SetupTest() {
	suite.exec = new(mocks.Executor)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.blobRepo = new(mocks.MockBlobRepository)
	suite.caseReviewUsecase = new(mockCaseReviewUsecase)
	suite.workerRepo = new(mocks.MockCaseReviewWorkerRepository)

	suite.ctx = context.Background()
}

func (suite *CaseReviewWorkerTestSuite) makeWorker() *CaseReviewWorker {
	worker := NewCaseReviewWorker(
		suite.blobRepo,
		"test-bucket-url",
		suite.caseReviewUsecase,
		suite.executorFactory,
		suite.workerRepo,
		30*time.Second,
	)

	return &worker
}

func (suite *CaseReviewWorkerTestSuite) AssertExpectations() {
	t := suite.T()
	suite.blobRepo.AssertExpectations(t)
	suite.caseReviewUsecase.AssertExpectations(t)
	suite.workerRepo.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.exec.AssertExpectations(t)
}

// Test helper to create test data
func createTestCaseReviewData() (models.CaseReviewArgs, models.Case, models.Organization, models.AiCaseReview) {
	caseId := uuid.New()
	aiCaseReviewId := uuid.New()

	args := models.CaseReviewArgs{
		CaseId:         caseId,
		AiCaseReviewId: aiCaseReviewId,
	}

	testCase := models.Case{
		Id:             caseId.String(),
		OrganizationId: "test-org-id",
	}

	org := models.Organization{
		Id:                  "test-org-id",
		AiCaseReviewEnabled: true,
	}

	aiCaseReview := models.AiCaseReview{
		Id:                aiCaseReviewId,
		CaseId:            caseId,
		Status:            models.AiCaseReviewStatusPending,
		FileReference:     "ai_case_reviews/final/test/file.json",
		FileTempReference: "ai_case_reviews/temp/test/file.json",
		BucketName:        "test-bucket",
	}

	return args, testCase, org, aiCaseReview
}

// TestGetPreviousCaseReviewContext_NoFile tests that getPreviousCaseReviewContext returns a zero value struct if the blob returns no file (err)
func (suite *CaseReviewWorkerTestSuite) TestGetPreviousCaseReviewContext_NoFile() {
	worker := suite.makeWorker()
	ctx := context.Background()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Mock blob repository to return an error (file not found)
	suite.blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(models.Blob{}, errors.New("file not found"))

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(ctx, aiCaseReview)

	// Assertions
	suite.NoError(err, "getPreviousCaseReviewContext should not return an error when file is not found")
	suite.Equal(CaseReviewContext{}, result, "Should return zero value struct when file is not found")

	suite.AssertExpectations()
}

// TestGetPreviousCaseReviewContext_ValidFile tests that getPreviousCaseReviewContext returns the right struct with right values if the blob returns a JSON file
func (suite *CaseReviewWorkerTestSuite) TestGetPreviousCaseReviewContext_ValidFile() {
	worker := suite.makeWorker()
	ctx := context.Background()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Create test case review context
	expectedContext := CaseReviewContext{
		DataModelSummary:       utils.Ptr("test summary"),
		FieldsToReadPerTable:   map[string][]string{"table1": {"field1", "field2"}},
		RulesDefinitionsReview: utils.Ptr("test rules review"),
		RuleThresholds:         utils.Ptr("test thresholds"),
	}

	// Convert to JSON
	contextJSON, err := json.Marshal(expectedContext)
	suite.NoError(err)

	// Mock blob repository to return the JSON data
	mockReader := newMockReadCloser(string(contextJSON))
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}

	suite.blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(ctx, aiCaseReview)

	// Assertions
	suite.NoError(err, "getPreviousCaseReviewContext should not return an error for valid JSON")
	suite.Equal(expectedContext, result, "Should return the correct parsed context")
	suite.True(mockReader.closed, "ReadCloser should be closed")

	suite.AssertExpectations()
}

// TestGetPreviousCaseReviewContext_InvalidJSON tests that getPreviousCaseReviewContext returns an error for invalid JSON
func (suite *CaseReviewWorkerTestSuite) TestGetPreviousCaseReviewContext_InvalidJSON() {
	worker := suite.makeWorker()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Mock blob repository to return invalid JSON
	mockReader := newMockReadCloser("invalid json content")
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}

	suite.blobRepo.On("GetBlob", suite.ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(suite.ctx, aiCaseReview)

	// Assertions
	suite.Error(err, "getPreviousCaseReviewContext should return an error for invalid JSON")
	suite.Equal(CaseReviewContext{}, result, "Should return zero value struct on JSON parse error")
	suite.True(mockReader.closed, "ReadCloser should be closed")

	suite.AssertExpectations()
}

// TestWork_Success tests the successful flow where CreateCaseReviewSync works correctly
func (suite *CaseReviewWorkerTestSuite) TestWork_Success() {
	worker := suite.makeWorker()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	suite.executorFactory.On("NewExecutor").Return(suite.exec)

	suite.workerRepo.On("GetCaseById", suite.ctx, suite.exec, args.CaseId.String()).
		Return(testCase, nil)

	suite.workerRepo.On("GetCaseReviewById", suite.ctx, suite.exec, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext - return error to get empty context
	suite.blobRepo.On("GetBlob", suite.ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(models.Blob{}, errors.New("file not found"))

	// Mock successful case review creation
	expectedDto := agent_dto.CaseReviewV1{
		Ok:          true,
		Output:      "Case review completed successfully",
		SanityCheck: "All checks passed",
		Thought:     "Analysis complete",
		Version:     "v1",
		Proofs:      []agent_dto.CaseReviewProof{},
	}

	suite.caseReviewUsecase.On("CreateCaseReviewSync", suite.ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(expectedDto, nil)

	// Mock HasAiCaseReviewEnabled to return true
	suite.caseReviewUsecase.On("HasAiCaseReviewEnabled", suite.ctx, org.Id).
		Return(true, nil)

	// Mock blob storage for final result
	mockWriter := newMockWriteCloser()
	suite.blobRepo.On("OpenStream", suite.ctx, "test-bucket-url", aiCaseReview.FileReference, aiCaseReview.FileReference).
		Return(mockWriter, nil)

	// Mock successful status update
	suite.workerRepo.On("UpdateCaseReviewFile", suite.ctx, suite.exec, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusCompleted,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(suite.ctx, job)

	// Assertions
	suite.NoError(err, "Work should complete successfully")
	suite.True(mockWriter.closed, "Write stream should be closed")

	// Verify the JSON was written correctly
	var writtenDto agent_dto.CaseReviewV1
	err = json.Unmarshal(mockWriter.Bytes(), &writtenDto)
	suite.NoError(err, "Written JSON should be valid")
	suite.Equal(expectedDto, writtenDto, "Written DTO should match expected")

	// Verify all expectations
	suite.AssertExpectations()
}

// TestWork_CreateCaseReviewSyncError tests that when CreateCaseReviewSync returns an error, the status is updated to failed
func (suite *CaseReviewWorkerTestSuite) TestWork_CreateCaseReviewSyncError() {
	worker := suite.makeWorker()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks for initial calls
	suite.executorFactory.On("NewExecutor").Return(suite.exec)

	suite.workerRepo.On("GetCaseById", suite.ctx, suite.exec, args.CaseId.String()).
		Return(testCase, nil)

	suite.workerRepo.On("GetCaseReviewById", suite.ctx, suite.exec, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext - return some context
	expectedContext := CaseReviewContext{
		DataModelSummary: utils.Ptr("test summary"),
	}
	contextJSON, _ := json.Marshal(expectedContext)
	mockReader := newMockReadCloser(string(contextJSON))
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}
	suite.blobRepo.On("GetBlob", suite.ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Mock failed case review creation
	suite.caseReviewUsecase.On("CreateCaseReviewSync", suite.ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(nil, errors.New("AI service unavailable"))

	suite.caseReviewUsecase.On("HasAiCaseReviewEnabled", suite.ctx, org.Id).
		Return(true, nil)

	// Mock blob storage for context save (during error handling)
	mockWriter := newMockWriteCloser()
	suite.blobRepo.On("OpenStream", suite.ctx, "test-bucket-url",
		aiCaseReview.FileTempReference, aiCaseReview.FileTempReference).
		Return(mockWriter, nil)

	// Mock failed status update
	suite.workerRepo.On("UpdateCaseReviewFile", suite.ctx, suite.exec, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusFailed,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(suite.ctx, job)

	// Assertions
	suite.Error(err, "Work should return error when CreateCaseReviewSync fails")
	suite.Contains(err.Error(), "Error while generating case review",
		"Error should contain the expected message")
	suite.True(mockWriter.closed, "Write stream should be closed")
	suite.True(mockReader.closed, "Read stream should be closed")

	// Verify the context was saved during error handling
	var savedContext CaseReviewContext
	err = json.Unmarshal(mockWriter.Bytes(), &savedContext)
	suite.NoError(err, "Saved JSON should be valid")
	suite.Equal(expectedContext, savedContext, "Saved context should match expected")

	// Verify all expectations
	suite.AssertExpectations()
}

// TestWork_OrganizationNotEnabled tests that when AI case review is not enabled for the organization, the work completes without error
func (suite *CaseReviewWorkerTestSuite) TestWork_OrganizationNotEnabled() {
	worker := suite.makeWorker()

	args, testCase, org, _ := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	suite.executorFactory.On("NewExecutor").Return(suite.exec)

	suite.workerRepo.On("GetCaseById", suite.ctx, suite.exec, args.CaseId.String()).
		Return(testCase, nil)

	suite.caseReviewUsecase.On("HasAiCaseReviewEnabled", suite.ctx, org.Id).
		Return(false, nil)

	// Call the method under test
	err := worker.Work(suite.ctx, job)

	// Assertions
	suite.NoError(err, "Work should complete successfully when AI case review is disabled")

	suite.AssertExpectations()
}

// TestWork_GetCaseError tests error handling when GetCaseById fails
func (suite *CaseReviewWorkerTestSuite) TestWork_GetCaseError() {
	worker := suite.makeWorker()

	args, _, _, _ := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	suite.executorFactory.On("NewExecutor").Return(suite.exec)

	suite.workerRepo.On("GetCaseById", suite.ctx, suite.exec, args.CaseId.String()).
		Return(models.Case{}, errors.New("case not found"))

	// Call the method under test
	err := worker.Work(suite.ctx, job)

	// Assertions
	suite.Error(err, "Work should return error when GetCaseById fails")
	suite.Contains(err.Error(), "Error while getting case",
		"Error should contain the expected message")

	suite.AssertExpectations()
}

// TestWork_BlobStreamError tests error handling when blob stream opening fails
func (suite *CaseReviewWorkerTestSuite) TestWork_BlobStreamError() {
	worker := suite.makeWorker()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks for initial calls
	suite.executorFactory.On("NewExecutor").Return(suite.exec)

	suite.workerRepo.On("GetCaseById", suite.ctx, suite.exec, args.CaseId.String()).
		Return(testCase, nil)

	suite.workerRepo.On("GetCaseReviewById", suite.ctx, suite.exec, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext
	suite.blobRepo.On("GetBlob", suite.ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(models.Blob{}, errors.New("file not found"))

	// Mock successful case review creation
	expectedDto := agent_dto.CaseReviewV1{
		Ok:     true,
		Output: "Case review completed successfully",
	}

	suite.caseReviewUsecase.On("CreateCaseReviewSync", suite.ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(expectedDto, nil)

	suite.caseReviewUsecase.On("HasAiCaseReviewEnabled", suite.ctx, org.Id).
		Return(true, nil)

	// Mock blob storage failure for final result
	suite.blobRepo.On("OpenStream", suite.ctx, "test-bucket-url", aiCaseReview.FileReference, aiCaseReview.FileReference).
		Return(nil, errors.New("blob storage unavailable"))

	// Mock error handling - blob storage for context save
	mockWriter := newMockWriteCloser()
	suite.blobRepo.On("OpenStream", suite.ctx, "test-bucket-url",
		aiCaseReview.FileTempReference, aiCaseReview.FileTempReference).
		Return(mockWriter, nil)

	// Mock failed status update
	suite.workerRepo.On("UpdateCaseReviewFile", suite.ctx, suite.exec, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusFailed,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(suite.ctx, job)

	// Assertions
	suite.Error(err, "Work should return error when blob stream opening fails")
	suite.Contains(err.Error(), "Error while opening stream",
		"Error should contain the expected message")

	// Verify all expectations
	suite.AssertExpectations()
}

func TestCaseReviewWorker(t *testing.T) {
	suite.Run(t, new(CaseReviewWorkerTestSuite))
}
