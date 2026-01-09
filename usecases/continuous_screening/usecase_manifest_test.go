package continuous_screening

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type ContinuousScreeningManifestTestSuite struct {
	suite.Suite
	repository      *mocks.ContinuousScreeningRepository
	blobRepository  *mocks.MockBlobRepository
	executorFactory executor_factory.ExecutorFactoryStub

	ctx                          context.Context
	marbleBackendUrl             string
	continuousScreeningBucketUrl string
}

func (suite *ContinuousScreeningManifestTestSuite) SetupTest() {
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.blobRepository = new(mocks.MockBlobRepository)
	suite.executorFactory = executor_factory.NewExecutorFactoryStub()

	suite.ctx = context.Background()
	suite.marbleBackendUrl = "https://api.marble.test"
	suite.continuousScreeningBucketUrl = "gs://marble-continuous-screening-entities"
}

func (suite *ContinuousScreeningManifestTestSuite) makeUsecase() *ContinuousScreeningManifestUsecase {
	return NewContinuousScreeningManifestUsecase(
		suite.executorFactory,
		suite.repository,
		suite.blobRepository,
		suite.marbleBackendUrl,
		suite.continuousScreeningBucketUrl,
	)
}

func TestContinuousScreeningManifestTestSuite(t *testing.T) {
	suite.Run(t, new(ContinuousScreeningManifestTestSuite))
}

func (suite *ContinuousScreeningManifestTestSuite) TestGetContinuousScreeningCatalog() {
	// Setup
	org1Id := uuid.New()
	org2Id := uuid.New()

	datasetFiles := []models.ContinuousScreeningDatasetFile{
		{
			OrgId:   org1Id,
			Version: "v1-org1",
		},
		{
			OrgId:   org2Id,
			Version: "v1-org2",
		},
	}

	suite.repository.On("ListContinuousScreeningLatestFullFiles", suite.ctx, suite.executorFactory.NewExecutor()).
		Return(datasetFiles, nil)

	// Execute
	uc := suite.makeUsecase()
	catalog, err := uc.GetContinuousScreeningCatalog(suite.ctx)

	// Assert
	suite.NoError(err)
	suite.Len(catalog.Datasets, 2)

	// Helper to find dataset by name
	getDataset := func(name string) *models.CatalogDataset {
		for _, ds := range catalog.Datasets {
			if ds.Name == name {
				return &ds
			}
		}
		return nil
	}

	// Check org1 dataset
	dataset1Name := orgCustomDatasetName(org1Id)
	ds1 := getDataset(dataset1Name)
	suite.NotNil(ds1)
	suite.Equal("v1-org1", ds1.Version)
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/full", org1Id), ds1.EntitiesUrl)
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/delta", org1Id), ds1.DeltaUrl)

	// Check org2 dataset
	dataset2Name := orgCustomDatasetName(org2Id)
	ds2 := getDataset(dataset2Name)
	suite.NotNil(ds2)
	suite.Equal("v1-org2", ds2.Version)
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/full", org2Id), ds2.EntitiesUrl)
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/delta", org2Id), ds2.DeltaUrl)
}

func (suite *ContinuousScreeningManifestTestSuite) TestGetContinuousScreeningDeltaList() {
	// Setup
	orgId := uuid.New()

	delta1Id := uuid.New()
	delta2Id := uuid.New()
	deltas := []models.ContinuousScreeningDatasetFile{
		{
			Id:      delta1Id,
			OrgId:   orgId,
			Version: "v1-delta",
		},
		{
			Id:      delta2Id,
			OrgId:   orgId,
			Version: "v2-delta",
		},
	}

	suite.repository.On("ListContinuousScreeningLatestDeltaFiles", suite.ctx,
		suite.executorFactory.NewExecutor(), orgId, uint64(LatestDeltaFilesLimit)).
		Return(deltas, nil)

	// Execute
	uc := suite.makeUsecase()
	deltaList, err := uc.GetContinuousScreeningDeltaList(suite.ctx, orgId)

	// Assert
	suite.NoError(err)
	suite.Len(deltaList.Versions, 2)
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/delta/%s",
		orgId, delta1Id), deltaList.Versions["v1-delta"])
	suite.Equal(fmt.Sprintf("https://api.marble.test/screening-indexer/org/%s/delta/%s",
		orgId, delta2Id), deltaList.Versions["v2-delta"])
}

func (suite *ContinuousScreeningManifestTestSuite) TestGetContinuousScreeningCatalog_Empty() {
	// Setup
	suite.repository.On("ListContinuousScreeningLatestFullFiles", suite.ctx, suite.executorFactory.NewExecutor()).
		Return([]models.ContinuousScreeningDatasetFile{}, nil)

	// Execute
	uc := suite.makeUsecase()
	catalog, err := uc.GetContinuousScreeningCatalog(suite.ctx)

	// Assert
	suite.NoError(err)
	suite.Empty(catalog.Datasets)
}

func (suite *ContinuousScreeningManifestTestSuite) TestGetContinuousScreeningDeltaList_Empty() {
	// Setup
	orgId := uuid.New()

	suite.repository.On("ListContinuousScreeningLatestDeltaFiles", suite.ctx,
		suite.executorFactory.NewExecutor(), orgId, uint64(LatestDeltaFilesLimit)).
		Return([]models.ContinuousScreeningDatasetFile{}, nil)

	// Execute
	uc := suite.makeUsecase()
	deltaList, err := uc.GetContinuousScreeningDeltaList(suite.ctx, orgId)

	// Assert
	suite.NoError(err)
	suite.Empty(deltaList.Versions)
}

func (suite *ContinuousScreeningManifestTestSuite) TestGetContinuousScreeningDeltaList_Error() {
	// Setup
	orgId := uuid.New()

	suite.repository.On("ListContinuousScreeningLatestDeltaFiles", suite.ctx,
		suite.executorFactory.NewExecutor(), orgId, uint64(LatestDeltaFilesLimit)).
		Return([]models.ContinuousScreeningDatasetFile{}, fmt.Errorf("db error"))

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.GetContinuousScreeningDeltaList(suite.ctx, orgId)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get continuous screening deltas")
}
