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
	CaseStatusByDate(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.CaseStatusByDate, error)
	CaseStatusByInbox(
		ctx context.Context,
		exec repositories.Executor,
		filters analytics.CaseAnalyticsFilter,
	) ([]analytics.CaseStatusByInbox, error)
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
		return nil, nil
	}
	return cachedTimeSeriesQuery(ctx, uc, filters, "cases_created", uc.repository.CasesCreatedByTimeStats)
}

func (uc CaseAnalyticsUsecase) CasesFalsePositiveRateByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CasesFalsePositiveRate, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedTimeSeriesQuery(ctx, uc, filters, "cases_false_positive_rate",
		uc.repository.CasesFalsePositiveRateByTimeStats)
}

func (uc CaseAnalyticsUsecase) CasesDurationByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CasesDuration, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedTimeSeriesQuery(ctx, uc, filters, "cases_duration", uc.repository.CasesDurationByTimeStats)
}

func (uc CaseAnalyticsUsecase) SarDelayByTimeStats(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.SarDelay, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedTimeSeriesQuery(ctx, uc, filters, "sar_delay", uc.repository.SarDelayByTimeStats)
}

func (uc CaseAnalyticsUsecase) SarCompletedCount(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) (analytics.SarCompletedCount, error) {
	if !uc.license.Analytics {
		return analytics.SarCompletedCount{}, nil
	}
	return cachedScalarQuery(ctx, uc, filters, "sar_completed", uc.repository.SarCompletedCount)
}

func (uc CaseAnalyticsUsecase) OpenCasesByAge(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.OpenCasesByAge, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedScalarQuery(ctx, uc, filters, "open_cases_by_age", uc.repository.OpenCasesByAge)
}

func (uc CaseAnalyticsUsecase) SarDelayDistribution(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.SarDelayDistribution, error) {
	if !uc.license.Analytics {
		return nil, nil
	}

	return cachedScalarQuery(ctx, uc, filters, "sar_delay_distribution", uc.repository.SarDelayDistribution)
}

func (uc CaseAnalyticsUsecase) CaseStatusByDate(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CaseStatusByDate, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedTimeSeriesQuery(ctx, uc, filters, "case_status_by_date", uc.repository.CaseStatusByDate)
}

func (uc CaseAnalyticsUsecase) CaseStatusByInbox(
	ctx context.Context,
	filters dto.CaseAnalyticsFilters,
) ([]analytics.CaseStatusByInbox, error) {
	if !uc.license.Analytics {
		return nil, nil
	}
	return cachedScalarQuery(ctx, uc, filters, "case_status_by_inbox", uc.repository.CaseStatusByInbox)
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
