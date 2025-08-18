package ai_agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gocloud.dev/blob"

	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
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

type mockBlobRepository struct {
	mock.Mock
}

func (r *mockBlobRepository) GetBlob(ctx context.Context, bucketUrl, key string) (models.Blob, error) {
	args := r.Called(ctx, bucketUrl, key)
	return args.Get(0).(models.Blob), args.Error(1)
}

func (r *mockBlobRepository) OpenStream(ctx context.Context, bucketUrl, key string, fileName string) (io.WriteCloser, error) {
	args := r.Called(ctx, bucketUrl, key, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (r *mockBlobRepository) OpenStreamWithOptions(ctx context.Context, bucketUrl, key string, opts *blob.WriterOptions) (io.WriteCloser, error) {
	args := r.Called(ctx, bucketUrl, key, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.WriteCloser), args.Error(1)
}

func (r *mockBlobRepository) DeleteFile(ctx context.Context, bucketUrl, key string) error {
	args := r.Called(ctx, bucketUrl, key)
	return args.Error(0)
}

func (r *mockBlobRepository) GenerateSignedUrl(ctx context.Context, bucketUrl, key string) (string, error) {
	args := r.Called(ctx, bucketUrl, key)
	return args.String(0), args.Error(1)
}

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

type mockCaseReviewWorkerRepository struct {
	mock.Mock
}

func (r *mockCaseReviewWorkerRepository) CreateCaseReviewFile(
	ctx context.Context,
	exec repositories.Executor,
	caseReview models.AiCaseReview,
) error {
	args := r.Called(ctx, exec, caseReview)
	return args.Error(0)
}

func (r *mockCaseReviewWorkerRepository) GetCaseReviewById(
	ctx context.Context,
	exec repositories.Executor,
	aiCaseReviewId uuid.UUID,
) (models.AiCaseReview, error) {
	args := r.Called(ctx, exec, aiCaseReviewId)
	return args.Get(0).(models.AiCaseReview), args.Error(1)
}

func (r *mockCaseReviewWorkerRepository) UpdateCaseReviewFile(
	ctx context.Context,
	exec repositories.Executor,
	caseReviewId uuid.UUID,
	status models.UpdateAiCaseReview,
) error {
	args := r.Called(ctx, exec, caseReviewId, status)
	return args.Error(0)
}

func (r *mockCaseReviewWorkerRepository) ListCaseReviewFiles(
	ctx context.Context,
	exec repositories.Executor,
	caseId uuid.UUID,
) ([]models.AiCaseReview, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).([]models.AiCaseReview), args.Error(1)
}

func (r *mockCaseReviewWorkerRepository) GetCaseById(ctx context.Context,
	exec repositories.Executor, caseId string,
) (models.Case, error) {
	args := r.Called(ctx, exec, caseId)
	return args.Get(0).(models.Case), args.Error(1)
}

func (r *mockCaseReviewWorkerRepository) GetOrganizationById(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
) (models.Organization, error) {
	args := r.Called(ctx, exec, organizationId)
	return args.Get(0).(models.Organization), args.Error(1)
}

type mockExecutorFactory struct {
	mock.Mock
}

func (e *mockExecutorFactory) NewClientDbExecutor(ctx context.Context, organizationId string) (repositories.Executor, error) {
	args := e.Called(ctx, organizationId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(repositories.Executor), args.Error(1)
}

func (e *mockExecutorFactory) NewExecutor() repositories.Executor {
	args := e.Called()
	return args.Get(0).(repositories.Executor)
}

type mockExecutor struct {
	mock.Mock
}

func (e *mockExecutor) DatabaseSchema() models.DatabaseSchema {
	args := e.Called()
	return args.Get(0).(models.DatabaseSchema)
}

func (e *mockExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := e.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (e *mockExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	argList := e.Called(ctx, sql, args)
	if argList.Get(0) == nil {
		return nil, argList.Error(1)
	}
	return argList.Get(0).(pgx.Rows), argList.Error(1)
}

func (e *mockExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	argList := e.Called(ctx, sql, args)
	return argList.Get(0).(pgx.Row)
}

// Test setup helper
func setupCaseReviewWorkerTest() (*CaseReviewWorker, *mockBlobRepository, *mockCaseReviewUsecase, *mockCaseReviewWorkerRepository, *mockExecutorFactory, *mockExecutor) {
	blobRepo := &mockBlobRepository{}
	caseReviewUsecase := &mockCaseReviewUsecase{}
	workerRepo := &mockCaseReviewWorkerRepository{}
	executorFactory := &mockExecutorFactory{}
	mockExec := &mockExecutor{}

	worker := NewCaseReviewWorker(
		blobRepo,
		"test-bucket-url",
		caseReviewUsecase,
		executorFactory,
		workerRepo,
		30*time.Second,
	)

	return &worker, blobRepo, caseReviewUsecase, workerRepo, executorFactory, mockExec
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
func TestGetPreviousCaseReviewContext_NoFile(t *testing.T) {
	worker, blobRepo, _, _, _, _ := setupCaseReviewWorkerTest()
	ctx := context.Background()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Mock blob repository to return an error (file not found)
	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(models.Blob{}, errors.New("file not found"))

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(ctx, aiCaseReview)

	// Assertions
	assert.NoError(t, err, "getPreviousCaseReviewContext should not return an error when file is not found")
	assert.Equal(t, CaseReviewContext{}, result, "Should return zero value struct when file is not found")

	blobRepo.AssertExpectations(t)
}

// TestGetPreviousCaseReviewContext_ValidFile tests that getPreviousCaseReviewContext returns the right struct with right values if the blob returns a JSON file
func TestGetPreviousCaseReviewContext_ValidFile(t *testing.T) {
	worker, blobRepo, _, _, _, _ := setupCaseReviewWorkerTest()
	ctx := context.Background()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Create test case review context
	expectedContext := CaseReviewContext{
		DataModelSummary:       &[]string{"test summary"}[0],
		FieldsToReadPerTable:   map[string][]string{"table1": {"field1", "field2"}},
		RulesDefinitionsReview: &[]string{"test rules review"}[0],
		RuleThresholds:         &[]string{"test thresholds"}[0],
	}

	// Convert to JSON
	contextJSON, err := json.Marshal(expectedContext)
	assert.NoError(t, err)

	// Mock blob repository to return the JSON data
	mockReader := newMockReadCloser(string(contextJSON))
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}

	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(ctx, aiCaseReview)

	// Assertions
	assert.NoError(t, err, "getPreviousCaseReviewContext should not return an error for valid JSON")
	assert.Equal(t, expectedContext, result, "Should return the correct parsed context")
	assert.True(t, mockReader.closed, "ReadCloser should be closed")

	blobRepo.AssertExpectations(t)
}

// TestGetPreviousCaseReviewContext_InvalidJSON tests that getPreviousCaseReviewContext returns an error for invalid JSON
func TestGetPreviousCaseReviewContext_InvalidJSON(t *testing.T) {
	worker, blobRepo, _, _, _, _ := setupCaseReviewWorkerTest()
	ctx := context.Background()

	_, _, _, aiCaseReview := createTestCaseReviewData()

	// Mock blob repository to return invalid JSON
	mockReader := newMockReadCloser("invalid json content")
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}

	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Call the method under test
	result, err := worker.getPreviousCaseReviewContext(ctx, aiCaseReview)

	// Assertions
	assert.Error(t, err, "getPreviousCaseReviewContext should return an error for invalid JSON")
	assert.Equal(t, CaseReviewContext{}, result, "Should return zero value struct on JSON parse error")
	assert.True(t, mockReader.closed, "ReadCloser should be closed")

	blobRepo.AssertExpectations(t)
}

// TestWork_Success tests the successful flow where CreateCaseReviewSync works correctly
func TestWork_Success(t *testing.T) {
	worker, blobRepo, caseReviewUsecase, workerRepo, executorFactory, mockExecutor := setupCaseReviewWorkerTest()
	ctx := context.Background()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	executorFactory.On("NewExecutor").Return(mockExecutor)

	workerRepo.On("GetCaseById", ctx, mockExecutor, args.CaseId.String()).
		Return(testCase, nil)

	workerRepo.On("GetCaseReviewById", ctx, mockExecutor, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext - return error to get empty context
	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
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

	caseReviewUsecase.On("CreateCaseReviewSync", ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(expectedDto, nil)

	// Mock HasAiCaseReviewEnabled to return true
	caseReviewUsecase.On("HasAiCaseReviewEnabled", ctx, org.Id).
		Return(true, nil)

	// Mock blob storage for final result
	mockWriter := newMockWriteCloser()
	blobRepo.On("OpenStream", ctx, "test-bucket-url", aiCaseReview.FileReference, aiCaseReview.FileReference).
		Return(mockWriter, nil)

	// Mock successful status update
	workerRepo.On("UpdateCaseReviewFile", ctx, mockExecutor, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusCompleted,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(ctx, job)

	// Assertions
	assert.NoError(t, err, "Work should complete successfully")
	assert.True(t, mockWriter.closed, "Write stream should be closed")

	// Verify the JSON was written correctly
	var writtenDto agent_dto.CaseReviewV1
	err = json.Unmarshal(mockWriter.Bytes(), &writtenDto)
	assert.NoError(t, err, "Written JSON should be valid")
	assert.Equal(t, expectedDto, writtenDto, "Written DTO should match expected")

	// Verify all expectations
	blobRepo.AssertExpectations(t)
	caseReviewUsecase.AssertExpectations(t)
	workerRepo.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}

// TestWork_CreateCaseReviewSyncError tests that when CreateCaseReviewSync returns an error, the status is updated to failed
func TestWork_CreateCaseReviewSyncError(t *testing.T) {
	worker, blobRepo, caseReviewUsecase, workerRepo, executorFactory, mockExecutor := setupCaseReviewWorkerTest()
	ctx := context.Background()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks for initial calls
	executorFactory.On("NewExecutor").Return(mockExecutor)

	workerRepo.On("GetCaseById", ctx, mockExecutor, args.CaseId.String()).
		Return(testCase, nil)

	workerRepo.On("GetCaseReviewById", ctx, mockExecutor, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext - return some context
	expectedContext := CaseReviewContext{
		DataModelSummary: &[]string{"test summary"}[0],
	}
	contextJSON, _ := json.Marshal(expectedContext)
	mockReader := newMockReadCloser(string(contextJSON))
	blob := models.Blob{
		FileName:   aiCaseReview.FileTempReference,
		ReadCloser: mockReader,
	}
	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(blob, nil)

	// Mock failed case review creation
	caseReviewUsecase.On("CreateCaseReviewSync", ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(nil, errors.New("AI service unavailable"))

	caseReviewUsecase.On("HasAiCaseReviewEnabled", ctx, org.Id).
		Return(true, nil)

	// Mock blob storage for context save (during error handling)
	mockWriter := newMockWriteCloser()
	blobRepo.On("OpenStream", ctx, "test-bucket-url", aiCaseReview.FileTempReference, aiCaseReview.FileTempReference).
		Return(mockWriter, nil)

	// Mock failed status update
	workerRepo.On("UpdateCaseReviewFile", ctx, mockExecutor, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusFailed,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(ctx, job)

	// Assertions
	assert.Error(t, err, "Work should return error when CreateCaseReviewSync fails")
	assert.Contains(t, err.Error(), "Error while generating case review",
		"Error should contain the expected message")
	assert.True(t, mockWriter.closed, "Write stream should be closed")
	assert.True(t, mockReader.closed, "Read stream should be closed")

	// Verify the context was saved during error handling
	var savedContext CaseReviewContext
	err = json.Unmarshal(mockWriter.Bytes(), &savedContext)
	assert.NoError(t, err, "Saved JSON should be valid")
	assert.Equal(t, expectedContext, savedContext, "Saved context should match expected")

	// Verify all expectations
	blobRepo.AssertExpectations(t)
	caseReviewUsecase.AssertExpectations(t)
	workerRepo.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}

// TestWork_OrganizationNotEnabled tests that when AI case review is not enabled for the organization, the work completes without error
func TestWork_OrganizationNotEnabled(t *testing.T) {
	worker, _, caseReviewUsecase, workerRepo, executorFactory, mockExecutor := setupCaseReviewWorkerTest()
	ctx := context.Background()

	args, testCase, org, _ := createTestCaseReviewData()
	org.AiCaseReviewEnabled = false // Disable AI case review

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	executorFactory.On("NewExecutor").Return(mockExecutor)

	workerRepo.On("GetCaseById", ctx, mockExecutor, args.CaseId.String()).
		Return(testCase, nil)

	caseReviewUsecase.On("HasAiCaseReviewEnabled", ctx, testCase.OrganizationId).
		Return(false, nil)

	// Call the method under test
	err := worker.Work(ctx, job)

	// Assertions
	assert.NoError(t, err, "Work should complete successfully when AI case review is disabled")

	// Verify expectations - should only call the initial methods
	workerRepo.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}

// TestWork_GetCaseError tests error handling when GetCaseById fails
func TestWork_GetCaseError(t *testing.T) {
	worker, _, _, workerRepo, executorFactory, mockExecutor := setupCaseReviewWorkerTest()
	ctx := context.Background()

	args, _, _, _ := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks
	executorFactory.On("NewExecutor").Return(mockExecutor)

	workerRepo.On("GetCaseById", ctx, mockExecutor, args.CaseId.String()).
		Return(models.Case{}, errors.New("case not found"))

	// Call the method under test
	err := worker.Work(ctx, job)

	// Assertions
	assert.Error(t, err, "Work should return error when GetCaseById fails")
	assert.Contains(t, err.Error(), "Error while getting case",
		"Error should contain the expected message")

	// Verify expectations
	workerRepo.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}

// TestWork_BlobStreamError tests error handling when blob stream opening fails
func TestWork_BlobStreamError(t *testing.T) {
	worker, blobRepo, caseReviewUsecase, workerRepo, executorFactory, mockExecutor := setupCaseReviewWorkerTest()
	ctx := context.Background()

	args, testCase, org, aiCaseReview := createTestCaseReviewData()

	job := &river.Job[models.CaseReviewArgs]{
		Args: args,
	}

	// Setup mocks for initial calls
	executorFactory.On("NewExecutor").Return(mockExecutor)

	workerRepo.On("GetCaseById", ctx, mockExecutor, args.CaseId.String()).
		Return(testCase, nil)

	workerRepo.On("GetCaseReviewById", ctx, mockExecutor, args.AiCaseReviewId).
		Return(aiCaseReview, nil)

	// Mock getPreviousCaseReviewContext
	blobRepo.On("GetBlob", ctx, "test-bucket-url", aiCaseReview.FileTempReference).
		Return(models.Blob{}, errors.New("file not found"))

	// Mock successful case review creation
	expectedDto := agent_dto.CaseReviewV1{
		Ok:     true,
		Output: "Case review completed successfully",
	}

	caseReviewUsecase.On("CreateCaseReviewSync", ctx, args.CaseId.String(),
		mock.AnythingOfType("*ai_agent.CaseReviewContext")).
		Return(expectedDto, nil)

	caseReviewUsecase.On("HasAiCaseReviewEnabled", ctx, org.Id).
		Return(true, nil)

	// Mock blob storage failure for final result
	blobRepo.On("OpenStream", ctx, "test-bucket-url", aiCaseReview.FileReference, aiCaseReview.FileReference).
		Return(nil, errors.New("blob storage unavailable"))

	// Mock error handling - blob storage for context save
	mockWriter := newMockWriteCloser()
	blobRepo.On("OpenStream", ctx, "test-bucket-url", aiCaseReview.FileTempReference, aiCaseReview.FileTempReference).
		Return(mockWriter, nil)

	// Mock failed status update
	workerRepo.On("UpdateCaseReviewFile", ctx, mockExecutor, aiCaseReview.Id, models.UpdateAiCaseReview{
		Status: models.AiCaseReviewStatusFailed,
	}).Return(nil)

	// Call the method under test
	err := worker.Work(ctx, job)

	// Assertions
	assert.Error(t, err, "Work should return error when blob stream opening fails")
	assert.Contains(t, err.Error(), "Error while opening stream",
		"Error should contain the expected message")

	// Verify all expectations
	blobRepo.AssertExpectations(t)
	caseReviewUsecase.AssertExpectations(t)
	workerRepo.AssertExpectations(t)
	executorFactory.AssertExpectations(t)
}
