package v1

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

var casePaginationDefaults = models.PaginationDefaults{
	Limit:  50,
	SortBy: models.CasesSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func HandleListCases(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p params.ListCasesParams

		if err := c.ShouldBindQuery(&p); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if !p.StartDate.IsZero() && !p.EndDate.IsZero() {
			if time.Time(p.StartDate).After(time.Time(p.EndDate)) {
				pubapi.NewErrorResponse().WithError(errors.WithDetail(
					pubapi.ErrInvalidPayload, "end date should be after start date")).Serve(c)
				return
			}
		}

		filters, err := p.ToFilters().Parse()
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		filters.UseLinearOrdering = true

		paging := p.PaginationParams.ToModel(casePaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cases, err := caseUsecase.ListCases(ctx, orgId, paging, filters)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		caseIds := pure_utils.Map(cases.Cases, func(cas models.Case) string { return cas.Id })

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, caseIds)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""

		if len(cases.Cases) > 0 {
			nextPageId = cases.Cases[len(cases.Cases)-1].Id
		}

		pubapi.
			NewResponse(pure_utils.Map(cases.Cases, dto.AdaptCase(users, tags, referents))).
			WithPagination(cases.HasNextPage, nextPageId).
			Serve(c)
	}
}

func HandleGetCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := pubapi.UuidParam(c, "caseId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cas, err := caseUsecase.GetCase(ctx, caseId.String())
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		pubapi.
			NewResponse(dto.AdaptCase(users, tags, referents)(cas)).
			Serve(c)
	}
}
