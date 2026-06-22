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

	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID) (models.Organization, error)
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
	serveFilesDirectly           bool
}

func NewContinuousScreeningManifestUsecase(
	executorFactory executor_factory.ExecutorFactory,
	repository ContinuousScreeningManifestRepository,
	blobRepository repositories.BlobRepository,
	marbleBackendUrl string,
	continuousScreeningBucketUrl string,
	serveFilesDirectly bool,
) *ContinuousScreeningManifestUsecase {
	return &ContinuousScreeningManifestUsecase{
		executorFactory:              executorFactory,
		repository:                   repository,
		blobRepository:               blobRepository,
		marbleBackendUrl:             marbleBackendUrl,
		continuousScreeningBucketUrl: continuousScreeningBucketUrl,
		serveFilesDirectly:           serveFilesDirectly,
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
	catalog.Datasets = make([]models.CatalogDataset, len(datasetFiles))

	for i, datasetFile := range datasetFiles {
		catalog.Datasets[i] = models.CatalogDataset{
			Name:        orgCustomDatasetName(datasetFile.OrgId),
			Title:       orgCustomDatasetName(datasetFile.OrgId),
			EntitiesUrl: datasetFileUrlBuilder(u.marbleBackendUrl, datasetFile.OrgId),
			Version:     datasetFile.Version,
			DeltaUrl:    deltaFileUrlBuilder(u.marbleBackendUrl, datasetFile.OrgId),
			Tags:        []string{MarbleContinuousScreeningTag},
		}
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

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningFull(
	ctx context.Context,
	orgId uuid.UUID,
) (models.ContinuousScreeningFileResult, error) {
	exec := u.executorFactory.NewExecutor()

	fullFile, err := u.repository.GetContinuousScreeningLatestDatasetFileByOrgId(ctx, exec, orgId,
		models.ContinuousScreeningDatasetFileTypeFull)
	if err != nil {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to get latest full dataset file")
	}
	if fullFile == nil {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(models.NotFoundError, "no full dataset file found for organization")
	}

	if u.serveFilesDirectly {
		blob, err := u.blobRepository.GetBlob(ctx, u.continuousScreeningBucketUrl, fullFile.FilePath)
		if err != nil {
			return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to read full dataset file blob")
		}
		return models.ContinuousScreeningFileResult{Blob: &blob}, nil
	}

	url, err := u.blobRepository.GenerateSignedUrl(ctx, u.continuousScreeningBucketUrl, fullFile.FilePath)
	if err != nil {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to generate signed url for full dataset file")
	}
	return models.ContinuousScreeningFileResult{RedirectURL: url}, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningDelta(
	ctx context.Context,
	orgId uuid.UUID,
	deltaId uuid.UUID,
) (models.ContinuousScreeningFileResult, error) {
	exec := u.executorFactory.NewExecutor()

	delta, err := u.repository.GetContinuousScreeningDatasetFileById(ctx, exec, deltaId)
	if err != nil {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to get continuous screening delta")
	}
	if delta.OrgId != orgId {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(models.ForbiddenError, "delta does not belong to the organization")
	}

	if u.serveFilesDirectly {
		blob, err := u.blobRepository.GetBlob(ctx, u.continuousScreeningBucketUrl, delta.FilePath)
		if err != nil {
			return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to read delta file blob")
		}
		return models.ContinuousScreeningFileResult{Blob: &blob}, nil
	}

	url, err := u.blobRepository.GenerateSignedUrl(ctx, u.continuousScreeningBucketUrl, delta.FilePath)
	if err != nil {
		return models.ContinuousScreeningFileResult{}, errors.Wrap(err, "failed to generate signed url for delta file")
	}
	return models.ContinuousScreeningFileResult{RedirectURL: url}, nil
}
