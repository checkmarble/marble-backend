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
	GetOrganizationIdByPublicId(ctx context.Context, exec repositories.Executor, publicOrgId uuid.UUID) (models.Organization, error)
	GetContinuousScreeningDatasetFileById(ctx context.Context, exec repositories.Executor,
		id uuid.UUID) (models.ContinuousScreeningDatasetFile, error)
	GetContinuousScreeningLatestDatasetFileByOrgId(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, fileType models.ContinuousScreeningDatasetFileType) (
		*models.ContinuousScreeningDatasetFile, error)
}

type ContinuousScreeningManifestUsecase struct {
	executorFactory  executor_factory.ExecutorFactory
	repository       ContinuousScreeningManifestRepository
	blobRepository   repositories.BlobRepository
	marbleBackendUrl string
	datasetBucketUrl string
}

func NewContinuousScreeningManifestUsecase(
	executorFactory executor_factory.ExecutorFactory,
	repository ContinuousScreeningManifestRepository,
	blobRepository repositories.BlobRepository,
	marbleBackendUrl string,
	datasetBucketUrl string,
) *ContinuousScreeningManifestUsecase {
	return &ContinuousScreeningManifestUsecase{
		executorFactory:  executorFactory,
		repository:       repository,
		blobRepository:   blobRepository,
		marbleBackendUrl: marbleBackendUrl,
		datasetBucketUrl: datasetBucketUrl,
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
		org, err := u.repository.GetOrganizationById(ctx, exec, datasetFile.OrgId.String())
		if err != nil {
			return models.CatalogResponse{}, errors.Wrap(err, "failed to get organization by id")
		}

		catalog.UpsertDataset(
			orgCustomDatasetName(org.PublicId),
			datasetFile.Version,
			datasetFileUrlBuilder(u.marbleBackendUrl, org.PublicId),
			deltaFileUrlBuilder(u.marbleBackendUrl, org.PublicId),
			[]string{"marble_continuous_screening"},
		)
	}

	return catalog, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningDeltaList(
	ctx context.Context,
	publicOrgId uuid.UUID,
) (models.ContinuousScreeningDeltaList, error) {
	exec := u.executorFactory.NewExecutor()
	org, err := u.repository.GetOrganizationIdByPublicId(ctx, exec, publicOrgId)
	if err != nil {
		return models.ContinuousScreeningDeltaList{},
			errors.Wrap(err, "failed to get organization id by public id")
	}
	orgId, err := uuid.Parse(org.Id)
	if err != nil {
		// Should never happen
		return models.ContinuousScreeningDeltaList{},
			errors.Wrap(err, "failed to parse organization id")
	}

	deltas, err := u.repository.ListContinuousScreeningLatestDeltaFiles(ctx, exec, orgId, LatestDeltaFilesLimit)
	if err != nil {
		return models.ContinuousScreeningDeltaList{},
			errors.Wrap(err, "failed to get continuous screening deltas")
	}

	versions := make(map[string]string)
	for _, delta := range deltas {
		versions[delta.Version] = deltaFileVersionUrlBuilder(u.marbleBackendUrl, org.PublicId, delta.Id)
	}

	return models.ContinuousScreeningDeltaList{
		Versions: versions,
	}, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningDeltaBlob(
	ctx context.Context,
	publicOrgId uuid.UUID,
	deltaId uuid.UUID,
) (models.Blob, error) {
	exec := u.executorFactory.NewExecutor()
	org, err := u.repository.GetOrganizationIdByPublicId(ctx, exec, publicOrgId)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get organization id by public id")
	}

	delta, err := u.repository.GetContinuousScreeningDatasetFileById(ctx, exec, deltaId)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get continuous screening delta")
	}

	if delta.OrgId.String() != org.Id {
		return models.Blob{},
			errors.New("delta does not belong to the organization")
	}

	blob, err := u.blobRepository.GetBlob(ctx, u.datasetBucketUrl, delta.FilePath)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get delta file blob")
	}

	return blob, nil
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningFullBlob(
	ctx context.Context,
	publicOrgId uuid.UUID,
) (models.Blob, error) {
	exec := u.executorFactory.NewExecutor()
	org, err := u.repository.GetOrganizationIdByPublicId(ctx, exec, publicOrgId)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get organization id by public id")
	}
	orgId, err := uuid.Parse(org.Id)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to parse organization id")
	}

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

	blob, err := u.blobRepository.GetBlob(ctx, u.datasetBucketUrl, fullFile.FilePath)
	if err != nil {
		return models.Blob{},
			errors.Wrap(err, "failed to get full dataset file blob")
	}

	return blob, nil
}
