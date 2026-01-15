package continuous_screening

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/h2non/gock"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func TestNewlineCountingWriter(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectedCount int
		description   string
	}{
		{
			name:          "single newline",
			input:         []byte("hello\nworld"),
			expectedCount: 1,
			description:   "should count one actual newline character",
		},
		{
			name:          "multiple newlines",
			input:         []byte("line1\nline2\nline3\nline4"),
			expectedCount: 3,
			description:   "should count three newline characters",
		},
		{
			name:          "escaped newline string literal",
			input:         []byte("hello\\nworld"),
			expectedCount: 0,
			description:   "should NOT count escaped newline (\\n as two characters), only actual newlines",
		},
		{
			name:          "mix of actual newlines and escaped newlines",
			input:         []byte("actual\nnewline and escaped\\nstring with\nanother actual"),
			expectedCount: 2,
			description:   "should count only actual newlines (2), not the escaped one (\\n)",
		},
		{
			name:          "newline at start",
			input:         []byte("\nhello"),
			expectedCount: 1,
			description:   "should count newline at the beginning",
		},
		{
			name:          "newline at end",
			input:         []byte("hello\n"),
			expectedCount: 1,
			description:   "should count newline at the end",
		},
		{
			name:          "consecutive newlines",
			input:         []byte("hello\n\n\nworld"),
			expectedCount: 3,
			description:   "should count each newline separately",
		},
		{
			name:          "no newlines",
			input:         []byte("hello world"),
			expectedCount: 0,
			description:   "should count zero newlines when there are none",
		},
		{
			name:          "empty input",
			input:         []byte(""),
			expectedCount: 0,
			description:   "should handle empty input gracefully",
		},
		{
			name:          "carriage return should not be counted",
			input:         []byte("hello\r\nworld"),
			expectedCount: 1,
			description:   "should count only the \\n in \\r\\n, not the \\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture written data
			var buf bytes.Buffer

			// Create the newlineCountingWriter
			writer := &newlineCountingWriter{writer: &buf}

			// Write the input
			n, err := writer.Write(tt.input)
			// Verify no error occurred
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify correct number of bytes were written
			if n != len(tt.input) {
				t.Errorf("expected %d bytes written, got %d", len(tt.input), n)
			}

			// Verify the correct count of newlines
			if writer.count != tt.expectedCount {
				t.Errorf("expected %d newlines, got %d | %s", tt.expectedCount, writer.count, tt.description)
			}

			// Verify the data was actually written to the underlying buffer
			if !bytes.Equal(buf.Bytes(), tt.input) {
				t.Errorf("expected written data to be %q, got %q", tt.input, buf.Bytes())
			}
		})
	}
}

func TestNewlineCountingWriter_MultipleWrites(t *testing.T) {
	// Test that the count accumulates across multiple Write calls
	var buf bytes.Buffer
	writer := &newlineCountingWriter{writer: &buf}

	// First write: 2 newlines
	n1, err := writer.Write([]byte("line1\nline2\n"))
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if writer.count != 2 {
		t.Errorf("after first write, expected count 2, got %d", writer.count)
	}

	// Second write: 1 newline
	n2, err := writer.Write([]byte("line3\nline4"))
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	if writer.count != 3 {
		t.Errorf("after second write, expected count 3, got %d", writer.count)
	}

	// Third write: 2 newlines
	n3, err := writer.Write([]byte("\n\nend"))
	if err != nil {
		t.Fatalf("third write failed: %v", err)
	}
	if writer.count != 5 {
		t.Errorf("after third write, expected count 5, got %d", writer.count)
	}

	// Verify total bytes written
	totalBytes := n1 + n2 + n3
	expectedTotalBytes := len([]byte("line1\nline2\n")) + len([]byte("line3\nline4")) + len([]byte("\n\nend"))
	if totalBytes != expectedTotalBytes {
		t.Errorf("expected %d total bytes, got %d", expectedTotalBytes, totalBytes)
	}

	// Verify all data was written correctly
	expected := []byte("line1\nline2\nline3\nline4\n\nend")
	if !bytes.Equal(buf.Bytes(), expected) {
		t.Errorf("expected buffer to contain %q, got %q", expected, buf.Bytes())
	}
}

type ScanDatasetUpdatesWorkerTestSuite struct {
	suite.Suite
	repository          *mocks.ContinuousScreeningRepository
	screeningProvider   *mocks.OpenSanctionsRepository
	blobRepo            *mocks.MockBlobRepository
	taskEnqueuer        *mocks.TaskQueueRepository
	featureAccessReader *mocks.FeatureAccessReader
	executorFactory     executor_factory.ExecutorFactoryStub
	transactionFactory  executor_factory.TransactionFactoryStub

	ctx context.Context
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.screeningProvider = new(mocks.OpenSanctionsRepository)
	suite.blobRepo = new(mocks.MockBlobRepository)
	suite.taskEnqueuer = new(mocks.TaskQueueRepository)
	suite.featureAccessReader = new(mocks.FeatureAccessReader)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) makeWorker() *ScanDatasetUpdatesWorker {
	return NewScanDatasetUpdatesWorker(
		suite.executorFactory,
		suite.transactionFactory,
		suite.repository,
		suite.screeningProvider,
		suite.blobRepo,
		suite.taskEnqueuer,
		suite.featureAccessReader,
		"test-bucket",
	)
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) AssertExpectations() {
	t := suite.T()
	suite.repository.AssertExpectations(t)
	suite.screeningProvider.AssertExpectations(t)
	suite.blobRepo.AssertExpectations(t)
	suite.taskEnqueuer.AssertExpectations(t)
	suite.featureAccessReader.AssertExpectations(t)
}

func TestScanDatasetUpdatesWorker(t *testing.T) {
	suite.Run(t, new(ScanDatasetUpdatesWorkerTestSuite))
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_NewDataset_CreatesRecord_NoProcessing() {
	// Setup
	datasetName := "test-dataset"
	version := "2024-01-01"

	// Mock catalog with one dataset
	catalog := models.OpenSanctionsRawCatalog{
		Current:  []string{datasetName},
		Outdated: []string{},
		Datasets: map[string]models.OpenSanctionsRawDataset{
			datasetName: {
				Name:    datasetName,
				Version: version,
			},
		},
	}

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks
	suite.screeningProvider.On("GetRawCatalog", suite.ctx).Return(catalog, nil)
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return([]models.ContinuousScreeningConfig{
		{Id: uuid.New(), OrgId: uuid.New(), Enabled: true},
	}, nil)
	// GetLastProcessedVersion returns NotFoundError for new dataset
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, datasetName).Return(
		models.ContinuousScreeningDatasetUpdate{}, models.NotFoundError)
	// CreateContinuousScreeningDatasetUpdate should be called for the new dataset
	expectedInput := models.CreateContinuousScreeningDatasetUpdate{
		DatasetName: datasetName,
		Version:     version,
	}
	expectedUpdate := models.ContinuousScreeningDatasetUpdate{
		Id:          uuid.New(),
		DatasetName: datasetName,
		Version:     version,
	}
	suite.repository.On("CreateContinuousScreeningDatasetUpdate", suite.ctx, mock.Anything,
		expectedInput).Return(expectedUpdate, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_DatasetWithoutDeltaUrl_NoProcessing() {
	// Test case 3: Have 2 datasets to update, but one has a previous version than DB one, need to ignore this one

	dataset1Name := "dataset1"
	dataset1DbVersion := "2024-01-02"      // DB has newer version
	dataset1CatalogVersion := "2024-01-01" // Catalog has older version - should be ignored

	dataset2Name := "dataset2"
	dataset2DbVersion := "2024-01-03"      // DB has older version
	dataset2CatalogVersion := "2024-01-04" // Catalog has newer version - should be processed

	// Mock catalog with two datasets
	catalog := models.OpenSanctionsRawCatalog{
		Current:  []string{dataset1Name, dataset2Name},
		Outdated: []string{},
		Datasets: map[string]models.OpenSanctionsRawDataset{
			dataset1Name: {
				Name:    dataset1Name,
				Version: dataset1CatalogVersion, // Older than DB
			},
			dataset2Name: {
				Name:    dataset2Name,
				Version: dataset2CatalogVersion, // Newer than DB
			},
		},
	}

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks
	suite.screeningProvider.On("GetRawCatalog", suite.ctx).Return(catalog, nil)
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return([]models.ContinuousScreeningConfig{
		{Id: uuid.New(), OrgId: uuid.New(), Enabled: true},
	}, nil)

	// Dataset1: DB has newer version than catalog - should be ignored
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, dataset1Name).Return(
		models.ContinuousScreeningDatasetUpdate{Version: dataset1DbVersion}, nil)

	// Dataset2: DB has older version than catalog - should be processed but has no DeltaUrl so skipped
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, dataset2Name).Return(
		models.ContinuousScreeningDatasetUpdate{Version: dataset2DbVersion}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_NoNewVersions_NoProcessing() {
	// Test case 4: No new version, no process

	dataset1Name := "dataset1"
	dataset1Version := "2024-01-01"

	dataset2Name := "dataset2"
	dataset2Version := "2024-01-02"

	// Mock catalog with two datasets
	catalog := models.OpenSanctionsRawCatalog{
		Current:  []string{dataset1Name, dataset2Name},
		Outdated: []string{},
		Datasets: map[string]models.OpenSanctionsRawDataset{
			dataset1Name: {
				Name:    dataset1Name,
				Version: dataset1Version,
			},
			dataset2Name: {
				Name:    dataset2Name,
				Version: dataset2Version,
			},
		},
	}

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks
	suite.screeningProvider.On("GetRawCatalog", suite.ctx).Return(catalog, nil)
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return([]models.ContinuousScreeningConfig{
		{Id: uuid.New(), OrgId: uuid.New(), Enabled: true},
	}, nil)

	// Both datasets have the same version as in DB - no updates
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, dataset1Name).Return(
		models.ContinuousScreeningDatasetUpdate{Version: dataset1Version}, nil)
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, dataset2Name).Return(
		models.ContinuousScreeningDatasetUpdate{Version: dataset2Version}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_HappyPath_ProcessDatasetUpdates() {
	datasetName := "test-dataset"
	oldVersion := "2024-01-01"
	newVersion := "2024-01-04" // Latest version in catalog
	deltaUrl := "https://example.com/delta"

	// Versions to be processed: 2024-01-02, 2024-01-03, 2024-01-04
	versionsToProcess := []string{"2024-01-02", "2024-01-03", "2024-01-04"}

	// Mock catalog with one dataset that has a newer version
	catalog := models.OpenSanctionsRawCatalog{
		Current:  []string{datasetName},
		Outdated: []string{},
		Datasets: map[string]models.OpenSanctionsRawDataset{
			datasetName: {
				Name:     datasetName,
				Version:  newVersion,
				DeltaUrl: &deltaUrl,
			},
		},
	}

	// Mock delta list response with 3 versions
	deltaList := `{
		"versions": {
			"2024-01-02": "https://example.com/delta/2024-01-02.ndjson",
			"2024-01-03": "https://example.com/delta/2024-01-03.ndjson",
			"2024-01-04": "https://example.com/delta/2024-01-04.ndjson"
		}
	}`

	// Mock continuous screening configs
	config1Id := uuid.New()
	config2Id := uuid.New()
	org1Id := uuid.New()
	org2Id := uuid.New()
	configs := []models.ContinuousScreeningConfig{
		{Id: config1Id, OrgId: org1Id, Enabled: true},
		{Id: config2Id, OrgId: org2Id, Enabled: true},
	}

	// Expected dataset updates (one for each version)
	expectedDatasetUpdates := make([]models.ContinuousScreeningDatasetUpdate, len(versionsToProcess))
	expectedUpdateJobs := make([]models.ContinuousScreeningUpdateJob, len(versionsToProcess)*len(configs))

	for i, version := range versionsToProcess {
		blobKey := fmt.Sprintf("%s/%s/%s.ndjson", ProviderUpdatesFolderName, datasetName, version)
		expectedDatasetUpdates[i] = models.ContinuousScreeningDatasetUpdate{
			Id:            uuid.New(),
			DatasetName:   datasetName,
			Version:       version,
			DeltaFilePath: blobKey,
			TotalItems:    5, // Each mocked to have 5 lines
		}

		// Create jobs for each config for this dataset update
		expectedUpdateJobs[i*2] = models.ContinuousScreeningUpdateJob{
			Id:              uuid.New(),
			DatasetUpdateId: expectedDatasetUpdates[i].Id,
			ConfigId:        config1Id,
			OrgId:           org1Id,
		}
		expectedUpdateJobs[i*2+1] = models.ContinuousScreeningUpdateJob{
			Id:              uuid.New(),
			DatasetUpdateId: expectedDatasetUpdates[i].Id,
			ConfigId:        config2Id,
			OrgId:           org2Id,
		}
	}

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks
	suite.featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, mock.Anything).Return(
		models.OrganizationFeatureAccess{
			ContinuousScreening: models.Allowed,
		}, nil,
	)
	suite.screeningProvider.On("GetRawCatalog", suite.ctx).Return(catalog, nil)

	// Dataset has older version in DB
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, datasetName).Return(
		models.ContinuousScreeningDatasetUpdate{Version: oldVersion}, nil)

	// Mock HTTP client for delta list and delta files
	defer gock.Off()
	gock.New("https://example.com").
		Get("/delta").
		Reply(200).
		BodyString(deltaList)

	// Mock delta file downloads for all versions
	deltaFileContent := "line1\nline2\nline3\nline4\nline5\n"
	for _, version := range versionsToProcess {
		gock.New("https://example.com").
			Get("/delta/" + version + ".ndjson").
			Reply(200).
			BodyString(deltaFileContent)
	}

	// Mock blob storage for all versions
	for _, version := range versionsToProcess {
		blobKey := fmt.Sprintf("%s/%s/%s.ndjson", ProviderUpdatesFolderName, datasetName, version)
		suite.blobRepo.On("OpenStream", mock.Anything, "test-bucket", blobKey, blobKey).Return(&mockBlobWriter{}, nil)
	}

	// Mock database operations in transaction
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return(configs, nil)

	// Mock dataset update creation for each version
	for i, version := range versionsToProcess {
		blobKey := fmt.Sprintf("%s/%s/%s.ndjson", ProviderUpdatesFolderName, datasetName, version)
		suite.repository.On("CreateContinuousScreeningDatasetUpdate", mock.Anything, mock.Anything,
			models.CreateContinuousScreeningDatasetUpdate{
				DatasetName:   datasetName,
				Version:       version,
				DeltaFilePath: blobKey,
				TotalItems:    5,
			}).Return(expectedDatasetUpdates[i], nil)
	}

	// Mock update job creation for each dataset update and config combination
	for _, job := range expectedUpdateJobs {
		suite.repository.On("CreateContinuousScreeningUpdateJob", mock.Anything, mock.Anything,
			models.CreateContinuousScreeningUpdateJob{
				DatasetUpdateId: job.DatasetUpdateId,
				ConfigId:        job.ConfigId,
				OrgId:           job.OrgId,
			}).Return(job, nil)
	}

	// Mock task enqueuing for all jobs
	for _, job := range expectedUpdateJobs {
		suite.taskEnqueuer.On("EnqueueContinuousScreeningApplyDeltaFileTask",
			mock.Anything, mock.Anything, job.OrgId, job.Id).Return(nil)
	}

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_NoActiveConfigs_ReturnsEarly() {
	// Test case: No active continuous screening configs, should return early

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks - only ListContinuousScreeningConfigs should be called
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return(
		[]models.ContinuousScreeningConfig{}, nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScanDatasetUpdatesWorkerTestSuite) TestWork_OrgMissingFeature_SkipsEnqueueing() {
	datasetName := "test-dataset"
	oldVersion := "2024-01-01"
	newVersion := "2024-01-02"
	deltaUrl := "https://example.com/delta"

	catalog := models.OpenSanctionsRawCatalog{
		Current:  []string{datasetName},
		Outdated: []string{},
		Datasets: map[string]models.OpenSanctionsRawDataset{
			datasetName: {
				Name:     datasetName,
				Version:  newVersion,
				DeltaUrl: &deltaUrl,
			},
		},
	}

	deltaList := `{
		"versions": {
			"2024-01-02": "https://example.com/delta/2024-01-02.ndjson"
		}
	}`

	config1Id := uuid.New()
	config2Id := uuid.New()
	org1Id := uuid.New()
	org2Id := uuid.New() // This org does NOT have the feature
	configs := []models.ContinuousScreeningConfig{
		{Id: config1Id, OrgId: org1Id, Enabled: true},
		{Id: config2Id, OrgId: org2Id, Enabled: true},
	}

	// Expected dataset update
	blobKey := fmt.Sprintf("%s/%s/%s.ndjson", ProviderUpdatesFolderName, datasetName, newVersion)
	expectedDatasetUpdate := models.ContinuousScreeningDatasetUpdate{
		Id:            uuid.New(),
		DatasetName:   datasetName,
		Version:       newVersion,
		DeltaFilePath: blobKey,
		TotalItems:    1,
	}

	// Expected update job - only for org1 (org2 is skipped due to missing feature)
	expectedUpdateJob := models.ContinuousScreeningUpdateJob{
		Id:              uuid.New(),
		DatasetUpdateId: expectedDatasetUpdate.Id,
		ConfigId:        config1Id,
		OrgId:           org1Id,
	}

	// Job args
	job := &river.Job[models.ContinuousScreeningScanDatasetUpdatesArgs]{}

	// Setup mocks
	suite.screeningProvider.On("GetRawCatalog", suite.ctx).Return(catalog, nil)

	// Dataset has older version in DB
	suite.repository.On("GetLastProcessedVersion", suite.ctx, mock.Anything, datasetName).Return(
		models.ContinuousScreeningDatasetUpdate{Version: oldVersion}, nil)

	// Mock HTTP client for delta list and delta files
	defer gock.Off()
	gock.New("https://example.com").
		Get("/delta").
		Reply(200).
		BodyString(deltaList)

	// Mock delta file downloads
	deltaFileContent := "line1\n"
	gock.New("https://example.com").
		Get("/delta/" + newVersion + ".ndjson").
		Reply(200).
		BodyString(deltaFileContent)

	// Mock blob storage
	suite.blobRepo.On("OpenStream", mock.Anything, "test-bucket", blobKey, blobKey).Return(&mockBlobWriter{}, nil)

	// Mock database operations in transaction
	suite.repository.On("ListContinuousScreeningConfigs", mock.Anything, mock.Anything).Return(configs, nil)

	// Mock dataset update creation
	suite.repository.On("CreateContinuousScreeningDatasetUpdate", mock.Anything, mock.Anything,
		models.CreateContinuousScreeningDatasetUpdate{
			DatasetName:   datasetName,
			Version:       newVersion,
			DeltaFilePath: blobKey,
			TotalItems:    1,
		}).Return(expectedDatasetUpdate, nil)

	// Mock feature access check: org1 has feature, org2 doesn't
	// org1 - feature allowed
	suite.featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, org1Id, mock.Anything).Return(
		models.OrganizationFeatureAccess{
			ContinuousScreening: models.Allowed,
		}, nil,
	)
	// org2 - feature not allowed (restricted)
	suite.featureAccessReader.On("GetOrganizationFeatureAccess", mock.Anything, org2Id, mock.Anything).Return(
		models.OrganizationFeatureAccess{
			ContinuousScreening: models.Restricted,
		}, nil,
	)

	// Mock update job creation - only for org1 (org2 is skipped)
	suite.repository.On("CreateContinuousScreeningUpdateJob", mock.Anything, mock.Anything,
		models.CreateContinuousScreeningUpdateJob{
			DatasetUpdateId: expectedDatasetUpdate.Id,
			ConfigId:        config1Id,
			OrgId:           org1Id,
		}).Return(expectedUpdateJob, nil)

	// Mock task enqueuing - only for org1 (org2 is skipped)
	suite.taskEnqueuer.On("EnqueueContinuousScreeningApplyDeltaFileTask",
		mock.Anything, mock.Anything, org1Id, expectedUpdateJob.Id).Return(nil)

	// Execute
	worker := suite.makeWorker()
	err := worker.Work(suite.ctx, job)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

// mockBlobWriter implements io.WriteCloser for testing
type mockBlobWriter struct {
	bytes.Buffer
}

func (m *mockBlobWriter) Close() error {
	return nil
}
