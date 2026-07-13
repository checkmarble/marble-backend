package continuous_screening

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestListContinuousScreeningClientDataIndexingUsesMotivaIndexVersion(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.MustParse("12345678-1234-1234-1234-123456789012")
	indexVersion := "20260713123000-001"
	repository := new(mocks.ContinuousScreeningRepository)
	screeningProvider := new(mocks.OpenSanctionsRepository)
	uc := &ContinuousScreeningUsecase{
		executorFactory:   executor_factory.NewExecutorFactoryStub(),
		repository:        repository,
		screeningProvider: screeningProvider,
	}
	pagination := models.PaginationAndSorting{
		Sorting: models.SortingFieldCreatedAt,
		Order:   models.SortingOrderDesc,
		Limit:   2,
	}
	org := models.Organization{
		Id: orgId,
		OpenSanctionsConfig: models.OrganizationOpenSanctionsConfig{
			Providers: map[models.ScreeningFeature]models.ScreeningProvider{
				models.ScreeningFeatureContinuousMonitoring: models.ScreeningProviderLexisNexis,
			},
		},
	}
	catalog := models.OpenSanctionsRawCatalog{
		Datasets: map[string]models.OpenSanctionsRawDataset{
			orgCustomDatasetName(orgId): {
				Version:      "20260713124500-001",
				IndexVersion: &indexVersion,
				IndexCurrent: false,
			},
		},
	}
	repositoryResult := models.ContinuousScreeningClientDataIndexing{
		PendingItems: 4,
		Items: models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{
			Items: []models.ContinuousScreeningClientDataIndexingSummary{
				{Version: "20260713120000-001"},
				{Version: "20260713121000-001"},
				{Version: "20260713122000-001"},
			},
		},
	}

	repository.On("GetOrganizationById", ctx, mock.Anything, orgId).Return(org, nil)
	screeningProvider.On("GetRawCatalog", ctx, models.ScreeningProviderLexisNexis).
		Return(catalog, nil)
	repository.On(
		"ListContinuousScreeningClientDataIndexing",
		ctx,
		mock.Anything,
		orgId,
		models.ScreeningProviderLexisNexis,
		mock.MatchedBy(func(version *string) bool {
			return version != nil && *version == indexVersion
		}),
		models.PaginationAndSorting{
			Sorting: models.SortingFieldCreatedAt,
			Order:   models.SortingOrderDesc,
			Limit:   3,
		},
	).Return(repositoryResult, nil)

	result, err := uc.ListContinuousScreeningClientDataIndexing(ctx, orgId, pagination)

	require.NoError(t, err)
	require.Equal(t, 4, result.PendingItems)
	require.Equal(t, "20260713124500-001", result.Version)
	require.Equal(t, &indexVersion, result.IndexVersion)
	require.False(t, result.IndexCurrent)
	require.Len(t, result.Items.Items, 2)
	require.True(t, result.Items.HasNextPage)
	repository.AssertExpectations(t)
	screeningProvider.AssertExpectations(t)
}

func TestListContinuousScreeningClientDataIndexingTreatsMissingMotivaDatasetAsUnindexed(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	repository := new(mocks.ContinuousScreeningRepository)
	screeningProvider := new(mocks.OpenSanctionsRepository)
	uc := &ContinuousScreeningUsecase{
		executorFactory:   executor_factory.NewExecutorFactoryStub(),
		repository:        repository,
		screeningProvider: screeningProvider,
	}
	pagination := models.PaginationAndSorting{
		Sorting: models.SortingFieldCreatedAt,
		Order:   models.SortingOrderDesc,
		Limit:   10,
	}
	org := models.Organization{Id: orgId}

	repository.On("GetOrganizationById", ctx, mock.Anything, orgId).Return(org, nil)
	screeningProvider.On("GetRawCatalog", ctx, models.ScreeningProviderOpenSanctions).
		Return(models.OpenSanctionsRawCatalog{
			Datasets: map[string]models.OpenSanctionsRawDataset{},
		}, nil)
	repository.On(
		"ListContinuousScreeningClientDataIndexing",
		ctx,
		mock.Anything,
		orgId,
		models.ScreeningProviderOpenSanctions,
		(*string)(nil),
		models.PaginationAndSorting{
			Sorting: models.SortingFieldCreatedAt,
			Order:   models.SortingOrderDesc,
			Limit:   11,
		},
	).Return(models.ContinuousScreeningClientDataIndexing{
		PendingItems: 7,
		Items: models.Paginated[models.ContinuousScreeningClientDataIndexingSummary]{
			Items: []models.ContinuousScreeningClientDataIndexingSummary{
				{Version: "20260713120000-001"},
			},
		},
	}, nil)

	result, err := uc.ListContinuousScreeningClientDataIndexing(ctx, orgId, pagination)

	require.NoError(t, err)
	require.Equal(t, 7, result.PendingItems)
	require.Empty(t, result.Version)
	require.Nil(t, result.IndexVersion)
	require.False(t, result.IndexCurrent)
	require.Len(t, result.Items.Items, 1)
	require.False(t, result.Items.HasNextPage)
	repository.AssertExpectations(t)
	screeningProvider.AssertExpectations(t)
}
