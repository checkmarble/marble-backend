package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type screeningEnforcerMock struct{}

func (screeningEnforcerMock) ReadDecision(models.Decision) error {
	return nil
}

func (screeningEnforcerMock) ReadOrUpdateCase(models.CaseMetadata, []uuid.UUID) error {
	return nil
}

type ScreeningCaseUsecaseMock struct {
	mock.Mock
}

func (m *ScreeningCaseUsecaseMock) PerformCaseActionSideEffects(ctx context.Context, tx repositories.Transaction, c models.Case) error {
	args := m.Called(ctx, tx, c)

	return args.Error(0)
}

type screeningRepositoryMock struct{}

func (screeningRepositoryMock) GetOrganizationById(ctx context.Context,
	exec repositories.Executor, organizationId uuid.UUID,
) (models.Organization, error) {
	return models.Organization{
		Id:   utils.TextToUUID("orgid"),
		Name: "ACME Inc.",
		OpenSanctionsConfig: models.OrganizationOpenSanctionsConfig{
			MatchThreshold: 42,
			MatchLimit:     10,
		},
	}, nil
}

func (screeningRepositoryMock) CreateCaseEvent(
	ctx context.Context,
	exec repositories.Executor,
	createCaseEventAttributes models.CreateCaseEventAttributes,
) (models.CaseEvent, error) {
	return models.CaseEvent{}, nil
}

func (screeningRepositoryMock) CreateCaseContributor(ctx context.Context, exec repositories.Executor, caseId, userId string) error {
	return nil
}

func (screeningRepositoryMock) DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error) {
	decisions := []models.Decision{
		{
			OrganizationId:      utils.TextToUUID("orgid"),
			DecisionId:          utils.TextToUUID("decisionid"),
			ScenarioIterationId: utils.TextToUUID("scenario-iteration-id"),
			Case:                &models.Case{},
		},
	}

	return decisions, nil
}

func (screeningRepositoryMock) ListInboxes(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
	withCaseCount bool,
) ([]models.Inbox, error) {
	parsedInboxId, _ := uuid.Parse("00000000-0000-0000-0000-000000000000") // Placeholder UUID
	inboxes := []models.Inbox{
		{Id: parsedInboxId},
	}

	return inboxes, nil
}
