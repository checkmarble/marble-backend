package params

import (
	"time"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type ListCasesParams struct {
	types.PaginationParams

	InboxIds   []string       `form:"inbox_id" binding:"omitempty,lte=1,dive,uuid"`
	Statuses   []string       `form:"status" binding:"omitempty,lte=1,dive,oneof=pending investigating closed"`
	AssignedTo string         `form:"assigned_to" binding:"omitempty,uuid"`
	StartDate  types.DateTime `form:"start" binding:"required_with=EndDate"`
	EndDate    types.DateTime `form:"end" binding:"required_with=StartDate"`
}

func (p ListCasesParams) ToFilters() gdto.CaseFilters {
	filters := gdto.CaseFilters{
		InboxIds:       p.InboxIds,
		Statuses:       p.Statuses,
		AssigneeId:     models.UserId(p.AssignedTo),
		IncludeSnoozed: true,
	}

	if !p.StartDate.IsZero() && !p.EndDate.IsZero() {
		filters.StartDate = time.Time(p.StartDate)
		filters.EndDate = time.Time(p.EndDate)
	}

	return filters
}
