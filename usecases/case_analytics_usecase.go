package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CaseAnalyticsRepository interface {
	CasesCreated(ctx context.Context, exec repositories.Executor,
		orgId uuid.UUID, inboxIds []uuid.UUID, assignedUserId *string,
		start, end time.Time, tzOffsetSeconds int,
	) ([]analytics.CasesCreated, error)
}

type CaseAnalyticsUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	inboxReader     inboxes.InboxReader
	license         models.LicenseValidation
	repository      CaseAnalyticsRepository
}

func (uc CaseAnalyticsUsecase) CasesCreated(ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CasesCreated, error) {
	if !uc.license.Analytics {
		return []analytics.CasesCreated{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.CasesCreated{}, nil
	}

	_, tzOffset := filters.End.In(filters.Timezone).Zone()

	return uc.repository.CasesCreated(ctx, exec,
		filters.OrgId, inboxIds, filters.AssignedUserId,
		filters.Start, filters.End, tzOffset)
}

func (uc CaseAnalyticsUsecase) getFilteredInboxIds(ctx context.Context,
	exec repositories.Executor, filters dto.CaseAnalyticsFilters,
) ([]uuid.UUID, error) {
	allInboxes, err := uc.inboxReader.ListInboxes(ctx, exec, filters.OrgId, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list available inboxes")
	}

	availableIds := make([]uuid.UUID, len(allInboxes))
	for i, inbox := range allInboxes {
		availableIds[i] = inbox.Id
	}

	if filters.InboxId != nil {
		for _, id := range availableIds {
			if id == *filters.InboxId {
				return []uuid.UUID{*filters.InboxId}, nil
			}
		}
		return []uuid.UUID{}, nil
	}

	return availableIds, nil
}
