package continuous_screening

import (
	"context"
	"encoding/json"
	"os"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/cockroachdb/errors"
)

type ContinuousScreeningManifestRepository interface {
	ListContinuousScreeningLatestDatasetFiles(ctx context.Context, exec repositories.Executor) (
		[]models.ContinuousScreeningDatasetFile,
		error,
	)

	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type ContinuousScreeningManifestUsecase struct {
	executorFactory                executor_factory.ExecutorFactory
	repository                     ContinuousScreeningManifestRepository
	continuousScreeningManifestUrl string
	marbleBackendUrl               string
}

func NewContinuousScreeningManifestUsecase(
	executorFactory executor_factory.ExecutorFactory,
	repository ContinuousScreeningManifestRepository,
	continuousScreeningManifestUrl string,
	marbleBackendUrl string,
) *ContinuousScreeningManifestUsecase {
	return &ContinuousScreeningManifestUsecase{
		executorFactory:                executorFactory,
		repository:                     repository,
		continuousScreeningManifestUrl: continuousScreeningManifestUrl,
		marbleBackendUrl:               marbleBackendUrl,
	}
}

func (u *ContinuousScreeningManifestUsecase) GetContinuousScreeningManifest(ctx context.Context) (models.Manifest, error) {
	exec := u.executorFactory.NewExecutor()

	// Fetch the manifest file from configuration
	if u.continuousScreeningManifestUrl == "" {
		return models.Manifest{}, errors.New("continuous screening manifest URL is not set")
	}

	file, err := os.Open(u.continuousScreeningManifestUrl)
	if err != nil {
		return models.Manifest{}, errors.Wrap(err, "failed to open manifest file")
	}
	defer file.Close()

	var manifest models.Manifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return models.Manifest{}, errors.Wrap(err, "failed to decode manifest")
	}

	datasetFiles, err := u.repository.ListContinuousScreeningLatestDatasetFiles(ctx, exec)
	if err != nil {
		return models.Manifest{}, errors.Wrap(err, "failed to list continuous screening latest dataset files")
	}

	for _, datasetFile := range datasetFiles {
		org, err := u.repository.GetOrganizationById(ctx, exec, datasetFile.OrgId.String())
		if err != nil {
			return models.Manifest{}, errors.Wrap(err, "failed to get organization by id")
		}
		manifest.UpsertDataset(
			org.Id,
			orgCustomDatasetName(org.PublicId),
			datasetFile.Version,
			datasetFileUrlBuilder(u.marbleBackendUrl, org.PublicId),
			deltaFileUrlBuilder(u.marbleBackendUrl, org.PublicId),
			ManifestAuthTokenFieldName,
		)
	}

	return manifest, nil
}
