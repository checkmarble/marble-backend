package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

type sanctionCheckEnforcerMock struct{}

func (sanctionCheckEnforcerMock) ReadDecision(models.Decision) error {
	return nil
}

func (sanctionCheckEnforcerMock) ReadOrUpdateCase(models.Case, []string) error {
	return nil
}

type sanctionCheckRepositoryMock struct{}

func (sanctionCheckRepositoryMock) GetOrganizationById(ctx context.Context,
	exec repositories.Executor, organizationId string,
) (models.Organization, error) {
	return models.Organization{
		Id:   "orgid",
		Name: "ACME Inc.",
		OpenSanctionsConfig: models.OrganizationOpenSanctionsConfig{
			Datasets:       []string{"ds1", "ds2"},
			MatchThreshold: utils.Ptr(42),
			MatchLimit:     utils.Ptr(10),
		},
	}, nil
}

func (sanctionCheckRepositoryMock) DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error) {
	decisions := []models.Decision{
		{
			DecisionId: "decisionid",
		},
	}

	return decisions, nil
}
