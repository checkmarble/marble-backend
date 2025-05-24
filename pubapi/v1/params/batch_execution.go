package params

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/utils"
)

type ListBatchExecutionsParams struct {
	pubapi.PaginationParams

	ScenarioId *string `form:"scenario_id" binding:"omitzero,uuid"`
}

func (p ListBatchExecutionsParams) ToFilters(orgId string) models.ListScheduledExecutionsFilters {
	filters := models.ListScheduledExecutionsFilters{
		OrganizationId: orgId,
	}

	if !utils.NilOrZero(p.ScenarioId) {
		filters.ScenarioId = *p.ScenarioId
	}

	return filters
}
