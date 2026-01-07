package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type autoAssignmentCaseRepository interface {
	AssignCase(ctx context.Context, exec repositories.Executor, id string, userId *models.UserId) error
	CreateCaseEvent(ctx context.Context, exec repositories.Executor, createCaseEventAttributes models.CreateCaseEventAttributes) error
}

type autoAssignmentRepository interface {
	FindAutoAssignableUsers(ctx context.Context, exec repositories.Executor, orgId string, limit int) ([]models.UserWithCaseCount, error)
	FindNextAutoAssignableUserForInbox(ctx context.Context, exec repositories.Executor, orgId string, inboxId uuid.UUID, limit int) (*models.User, error)
	FindAutoAssignableCases(ctx context.Context, exec repositories.Executor, orgId string, limit int) ([]models.Case, error)
}

type autoAssignmentOrgRepository interface {
	GetOrganizationById(ctx context.Context, exec repositories.Executor, organizationId string) (models.Organization, error)
}

type AutoAssignmentUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	caseRepository     autoAssignmentCaseRepository
	orgRepository      autoAssignmentOrgRepository
	repository         autoAssignmentRepository
}

func (uc AutoAssignmentUsecase) RunAutoAssigner(ctx context.Context, orgId string, inboxId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)

	org, err := uc.orgRepository.GetOrganizationById(ctx, uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return errors.Wrap(err, "could not retrieve organization settings")
	}

	// Find maximum slots across inboxes (to limit how many cases to consider).
	assignableUsers, err := uc.repository.FindAutoAssignableUsers(ctx, uc.executorFactory.NewExecutor(), orgId, org.AutoAssignQueueLimit)
	if err != nil {
		return errors.Wrap(err, "could not find assignable users")
	}

	slots := 0
	for _, user := range assignableUsers {
		slots += org.AutoAssignQueueLimit - user.CaseCount
	}
	if slots == 0 {
		logger.DebugContext(ctx, "no auto-assignable user have any space in their queue, aborting.")
		return nil
	}

	logger.DebugContext(ctx, fmt.Sprintf("case auto-assignment: found %d empty queue slots", slots))

	cases, err := uc.repository.FindAutoAssignableCases(ctx, uc.executorFactory.NewExecutor(), orgId, slots)
	if err != nil {
		return errors.Wrap(err, "could not find assignable cases")
	}

	logger.DebugContext(ctx, fmt.Sprintf("case auto-assignment: found %d auto-assignable cases", len(cases)))

	for _, c := range cases {
		// Look for the user with the least number of assigned cases, who has access to the case's inbox
		user, err := uc.repository.FindNextAutoAssignableUserForInbox(ctx, uc.executorFactory.NewExecutor(), c.OrganizationId.String(), c.InboxId, org.AutoAssignQueueLimit)
		if err != nil {
			return err
		}

		// If no rows are returned, it means all users in that inbox's rotation are at capacity
		if user == nil {
			continue
		}

		logger.DebugContext(ctx, "auto-assigning case to user",
			"case_id", c.Id,
			"user_id", user.UserId)

		if err := uc.assignCase(ctx, c, *user); err != nil {
			return errors.Wrap(err, "could not assign case")
		}
	}

	return nil
}

func (uc AutoAssignmentUsecase) assignCase(ctx context.Context, c models.Case, user models.User) error {
	return uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.caseRepository.AssignCase(ctx, tx, c.Id, &user.UserId); err != nil {
			return err
		}

		if err := uc.caseRepository.CreateCaseEvent(ctx, tx, models.CreateCaseEventAttributes{
			OrgId:     c.OrganizationId,
			CaseId:    c.Id,
			UserId:    nil,
			EventType: models.CaseAssigned,
			NewValue:  (*string)(&user.UserId),
		}); err != nil {
			return err
		}

		return nil
	})
}
