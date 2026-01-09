package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

// Only return the latest 30 delta files for each org
const LatestDeltaFilesLimit = 30

type ContinuousScreeningManifestRepository interface {
	ListContinuousScreeningLatestFullFiles(ctx context.Context, exec repositories.Executor) (
		[]models.ContinuousScreeningDatasetFile,
		error,
	)
	ListContinuousScreeningLatestDeltaFiles(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, limit uint64) (
		[]models.ContinuousScreeningDatasetFile,
		error,
	)

	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
	GetContinuousScreeningDatasetFileById(ctx context.Context, exec repositories.Executor,
		id uuid.UUID) (models.ContinuousScreeningDatasetFile, error)
	GetContinuousScreeningLatestDatasetFileByOrgId(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, fileType models.ContinuousScreeningDatasetFileType) (
		*models.ContinuousScreeningDatasetFile, error)
}

type ContinuousScreeningManifestUsecase struct {
	executorFactory              executor_factory.ExecutorFactory
	repository                   ContinuousScreeningManifestRepository
	blobRepository               repositories.BlobRepository
	marbleBackendUrl             string
	continuousScreeningBucketUrl string
}

func NewContinuousScreeningManifestUsecase(
	executorFactory executor_factory.ExecutorFactory,
	repository ContinuousScreeningManifestRepository,
	blobRepository repositories.BlobRepository,
	marbleBackendUrl string,
	continuousScreeningBucketUrl string,
) *ContinuousScreeningManifestUsecase {
	return &ContinuousScreeningManifestUsecase{
		executorFactory:              executorFactory,
		repository:                   repository,
		blobRepository:               blobRepository,
		marbleBackendUrl:             marbleBackendUrl,
		continuousScreeningBucketUrl: continuousScreeningBucketUrl,
	}
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningCatalog(ctx context.Context) (models.CatalogResponse, error) {
	exec := u.executorFactory.NewExecutor()

	datasetFiles, err := u.repository.ListContinuousScreeningLatestFullFiles(ctx, exec)
	if err != nil {
		return models.CatalogResponse{}, errors.Wrap(err,
			"failed to list continuous screening latest dataset files")
	}

	var catalog models.CatalogResponse

	for _, datasetFile := range datasetFiles {
		catalog.UpsertDataset(
			orgCustomDatasetName(datasetFile.OrgId),
			datasetFile.Version,
			datasetFileUrlBuilder(u.marbleBackendUrl, datasetFile.OrgId),
			deltaFileUrlBuilder(u.marbleBackendUrl, datasetFile.OrgId),
			[]string{"marble_continuous_screening"},
		)
	}

	return catalog, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningDeltaList(
	ctx context.Context,
	orgId uuid.UUID,
) (models.ContinuousScreeningDeltaList, error) {
	exec := u.executorFactory.NewExecutor()

	deltas, err := u.repository.ListContinuousScreeningLatestDeltaFiles(ctx, exec, orgId, LatestDeltaFilesLimit)
	if err != nil {
		return models.ContinuousScreeningDeltaList{},
			errors.Wrap(err, "failed to get continuous screening deltas")
	}

	versions := make(map[string]string)
	for _, delta := range deltas {
		versions[delta.Version] = deltaFileVersionUrlBuilder(u.marbleBackendUrl, orgId, delta.Id)
	}

	return models.ContinuousScreeningDeltaList{
		Versions: versions,
	}, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningDeltaBlob(
	ctx context.Context,
	orgId uuid.UUID,
	deltaId uuid.UUID,
) (models.Blob, error) {
	exec := u.executorFactory.NewExecutor()

	delta, err := u.repository.GetContinuousScreeningDatasetFileById(ctx, exec, deltaId)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get continuous screening delta")
	}

	if delta.OrgId != orgId {
		return models.Blob{},
			errors.New("delta does not belong to the organization")
	}

	blob, err := u.blobRepository.GetBlob(ctx, u.continuousScreeningBucketUrl, delta.FilePath)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get delta file blob")
	}

	return blob, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningFullBlob(
	ctx context.Context,
	orgId uuid.UUID,
) (models.Blob, error) {
	exec := u.executorFactory.NewExecutor()

	fullFile, err := u.repository.GetContinuousScreeningLatestDatasetFileByOrgId(ctx, exec, orgId,
		models.ContinuousScreeningDatasetFileTypeFull)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get latest full dataset file")
	}

	if fullFile == nil {
		return models.Blob{},
			errors.Wrap(models.NotFoundError, "no full dataset file found for organization")
	}

	blob, err := u.blobRepository.GetBlob(ctx, u.continuousScreeningBucketUrl, fullFile.FilePath)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get full dataset file blob")
	}

	return blob, nil
}
