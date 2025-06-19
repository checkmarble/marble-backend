package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type sanctionCheckEnforcerMock struct{}

func (sanctionCheckEnforcerMock) ReadDecision(models.Decision) error {
	return nil
}

func (sanctionCheckEnforcerMock) ReadOrUpdateCase(models.CaseMetadata, []string) error {
	return nil
}

type SanctionCheckCaseUsecaseMock struct {
	mock.Mock
}

func (m *SanctionCheckCaseUsecaseMock) PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, c models.Case) error {
	args := m.Called(ctx, tx, c)

	return args.Error(0)
}

type sanctionCheckRepositoryMock struct{}

func (sanctionCheckRepositoryMock) GetOrganizationById(ctx context.Context,
	exec repositories.Executor, organizationId string,
) (models.Organization, error) {
	return models.Organization{
		Id:   "orgid",
		Name: "ACME Inc.",
		OpenSanctionsConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: 42,
			MatchLimit:     10,
		},
	}, nil
}

func (sanctionCheckRepositoryMock) CreateCaseEvent(
	ctx context.Context,
	exec repositories.Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) error {
	return nil
}

func (sanctionCheckRepositoryMock) CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error {
	return nil
}

func (sanctionCheckRepositoryMock) DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error) {
	decisions := []models.Decision{
		{
			OrganizationId: "orgid",
			DecisionId:     "decisionid",
			Case:           &models.Case{},
		},
	}

	return decisions, nil
}

func (sanctionCheckRepositoryMock) ListInboxes(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	withCaseCount bool,
) ([]models.Inbox, error) {
	parsedInboxId, _ := uuid.Parse("00000000-0000-0000-0000-000000000000") // Placeholder UUID
	inboxes := []models.Inbox{
		{Id: parsedInboxId},
	}

	return inboxes, nil
}
