package usecases

import (
	"context"
	"slices"

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
	CasesCreatedByTimeStats(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.CasesCreated, error)
	CasesFalsePositiveRateByTimeStats(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.CasesFalsePositiveRate, error)
	CasesDurationByTimeStats(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.CasesDuration, error)
	SarCompletedCount(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) (analytics.SarCompletedCount, error)
	OpenCasesByAge(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.OpenCasesByAge, error)
	SarDelayByTimeStats(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.SarDelay, error)
	SarDelayDistribution(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.SarDelayDistribution, error)
}

type CaseAnalyticsUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	inboxReader     inboxes.InboxReader
	license         models.LicenseValidation
	repository      CaseAnalyticsRepository
}

func (uc CaseAnalyticsUsecase) CasesCreatedByTimeStats(
	ctx context.Context,
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

	return uc.repository.CasesCreatedByTimeStats(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:           filters.OrgId,
		InboxIds:        inboxIds,
		AssignedUserId:  filters.AssignedUserId,
		Start:           filters.Start,
		End:             filters.End,
		TzOffsetSeconds: tzOffset,
	})
}

func (uc CaseAnalyticsUsecase) CasesFalsePositiveRateByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CasesFalsePositiveRate, error) {
	if !uc.license.Analytics {
		return []analytics.CasesFalsePositiveRate{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.CasesFalsePositiveRate{}, nil
	}

	_, tzOffset := filters.End.In(filters.Timezone).Zone()

	return uc.repository.CasesFalsePositiveRateByTimeStats(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:           filters.OrgId,
		InboxIds:        inboxIds,
		AssignedUserId:  filters.AssignedUserId,
		Start:           filters.Start,
		End:             filters.End,
		TzOffsetSeconds: tzOffset,
	})
}

func (uc CaseAnalyticsUsecase) CasesDurationByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CasesDuration, error) {
	if !uc.license.Analytics {
		return []analytics.CasesDuration{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.CasesDuration{}, nil
	}

	_, tzOffset := filters.End.In(filters.Timezone).Zone()

	return uc.repository.CasesDurationByTimeStats(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:           filters.OrgId,
		InboxIds:        inboxIds,
		AssignedUserId:  filters.AssignedUserId,
		Start:           filters.Start,
		End:             filters.End,
		TzOffsetSeconds: tzOffset,
	})
}

func (uc CaseAnalyticsUsecase) SarCompletedCount(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) (analytics.SarCompletedCount, error) {
	if !uc.license.Analytics {
		return analytics.SarCompletedCount{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return analytics.SarCompletedCount{}, err
	}
	if len(inboxIds) == 0 {
		return analytics.SarCompletedCount{}, nil
	}

	return uc.repository.SarCompletedCount(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:          filters.OrgId,
		InboxIds:       inboxIds,
		AssignedUserId: filters.AssignedUserId,
		Start:          filters.Start,
		End:            filters.End,
	})
}

func (uc CaseAnalyticsUsecase) OpenCasesByAge(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.OpenCasesByAge, error) {
	if !uc.license.Analytics {
		return []analytics.OpenCasesByAge{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.OpenCasesByAge{}, nil
	}

	return uc.repository.OpenCasesByAge(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:          filters.OrgId,
		InboxIds:       inboxIds,
		AssignedUserId: filters.AssignedUserId,
	})
}

func (uc CaseAnalyticsUsecase) SarDelayByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.SarDelay, error) {
	if !uc.license.Analytics {
		return []analytics.SarDelay{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.SarDelay{}, nil
	}

	_, tzOffset := filters.End.In(filters.Timezone).Zone()

	return uc.repository.SarDelayByTimeStats(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:           filters.OrgId,
		InboxIds:        inboxIds,
		AssignedUserId:  filters.AssignedUserId,
		Start:           filters.Start,
		End:             filters.End,
		TzOffsetSeconds: tzOffset,
	})
}

func (uc CaseAnalyticsUsecase) SarDelayDistribution(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.SarDelayDistribution, error) {
	if !uc.license.Analytics {
		return []analytics.SarDelayDistribution{}, nil
	}

	exec := uc.executorFactory.NewExecutor()

	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []analytics.SarDelayDistribution{}, nil
	}

	return uc.repository.SarDelayDistribution(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:          filters.OrgId,
		InboxIds:       inboxIds,
		AssignedUserId: filters.AssignedUserId,
		Start:          filters.Start,
		End:            filters.End,
	})
}

func (uc CaseAnalyticsUsecase) getFilteredInboxIds(
	ctx context.Context,
	exec repositories.Executor,
	filters dto.CaseAnalyticsFilters,
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
		if slices.Contains(availableIds, *filters.InboxId) {
			return []uuid.UUID{*filters.InboxId}, nil
		}
	}

	return availableIds, nil
}
