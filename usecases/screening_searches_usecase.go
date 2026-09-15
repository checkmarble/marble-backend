package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type screeningSavedSearchRepository interface {
	SaveScreeningSearch(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, providerName models.ScreeningProvider, name string, cfg dto.ScreeningFreeformDto) (models.ScreeningSavedSearch, error)
	ListSavedScreeningSearches(ctx context.Context, exec repositories.Executor, orgId uuid.UUID, providerName models.ScreeningProvider) ([]models.ScreeningSavedSearch, error)
	DeleteSavedSearch(ctx context.Context, exec repositories.Executor, orgId, id uuid.UUID) error
}

type ScreeningSearchesUsecase struct {
	enforceSecurity        ScreeningEnforceSecurity
	featureAccessReader    feature_access.FeatureAccessReader
	executorFactory        executor_factory.ExecutorFactory
	repository             screeningSavedSearchRepository
	organizationRepository repositories.OrganizationRepository
}

func NewScreeningSearchesUsecase(
	enforceSecurity ScreeningEnforceSecurity,
	featureAccessReader feature_access.FeatureAccessReader,
	executorFactory executor_factory.ExecutorFactory,
	repository screeningSavedSearchRepository,
	organizationRepository repositories.OrganizationRepository,
) ScreeningSearchesUsecase {
	return ScreeningSearchesUsecase{
		enforceSecurity:        enforceSecurity,
		featureAccessReader:    featureAccessReader,
		executorFactory:        executorFactory,
		repository:             repository,
		organizationRepository: organizationRepository,
	}
}

func (uc ScreeningSearchesUsecase) ListSearches(ctx context.Context) ([]models.ScreeningSavedSearch, error) {
	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	org, err := uc.organizationRepository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return nil, err
	}

	providerName := org.GetScreeningProviderFor(models.ScreeningFeatureManualSearch)

	features, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return nil, err
	}

	if !features.Sanctions.IsAllowed() && !features.ContinuousScreening.IsAllowed() {
		return nil, models.ForbiddenError
	}

	if err := uc.enforceSecurity.SaveScreeningSearch(); err != nil {
		return nil, err
	}

	return uc.repository.ListSavedScreeningSearches(ctx, exec, orgId, providerName)
}

func (uc ScreeningSearchesUsecase) SaveSearch(ctx context.Context, name string, cfg dto.ScreeningFreeformDto) (models.ScreeningSavedSearch, error) {
	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	org, err := uc.organizationRepository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return models.ScreeningSavedSearch{}, err
	}

	providerName := org.GetScreeningProviderFor(models.ScreeningFeatureManualSearch)

	features, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return models.ScreeningSavedSearch{}, err
	}

	if !features.Sanctions.IsAllowed() && !features.ContinuousScreening.IsAllowed() {
		return models.ScreeningSavedSearch{}, models.ForbiddenError
	}

	if err := uc.enforceSecurity.SaveScreeningSearch(); err != nil {
		return models.ScreeningSavedSearch{}, models.ForbiddenError
	}

	cfg.Query.ClearFields()

	search, err := uc.repository.SaveScreeningSearch(ctx, exec, orgId, providerName, name, cfg)
	if repositories.IsUniqueViolationError(err) {
		return models.ScreeningSavedSearch{}, errors.Wrap(models.ConflictError,
			"a search with that name already exists")
	}

	return search, err
}

func (uc ScreeningSearchesUsecase) DeleteSavedSearch(ctx context.Context, id uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()
	orgId := uc.enforceSecurity.OrgId()

	features, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return err
	}

	if !features.Sanctions.IsAllowed() && !features.ContinuousScreening.IsAllowed() {
		return models.ForbiddenError
	}

	if err := uc.enforceSecurity.SaveScreeningSearch(); err != nil {
		return err
	}

	return uc.repository.DeleteSavedSearch(ctx, exec, orgId, id)
}
