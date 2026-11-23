package continuous_screening

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"golang.org/x/time/rate"
)

const deltaSearchBurst = 1

type DeltaSearchRates struct {
	Production    float64
	NonProduction float64
}

type deltaSearchLimiters struct {
	production    *rate.Limiter
	nonProduction *rate.Limiter
}

func newDeltaSearchLimiters(rates DeltaSearchRates) deltaSearchLimiters {
	return deltaSearchLimiters{
		production:    rate.NewLimiter(rate.Limit(rates.Production), deltaSearchBurst),
		nonProduction: rate.NewLimiter(rate.Limit(rates.NonProduction), deltaSearchBurst),
	}
}

func (l deltaSearchLimiters) getLimiterForOrgEnv(env models.OrganizationEnvironment) *rate.Limiter {
	if env == models.OrganizationEnvironmentProduction {
		return l.production
	} else {
		return l.nonProduction
	}
}

// Wraps an applyDeltaFileWorkerScreeningProvider implementation by rate-limiting the searches
type rateLimitedDeltaSearch struct {
	provider applyDeltaFileWorkerScreeningProvider
	limiter  *rate.Limiter
}

func (s rateLimitedDeltaSearch) Search(
	ctx context.Context,
	providerName models.ScreeningProvider,
	query models.OpenSanctionsQuery,
) (models.ScreeningRawSearchResponseWithMatches, error) {
	if err := s.limiter.Wait(ctx); err != nil {
		return models.ScreeningRawSearchResponseWithMatches{}, err
	} else {
		return s.provider.Search(ctx, providerName, query)
	}
}
