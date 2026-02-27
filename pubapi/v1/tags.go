package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

var tagPaginationDefaults = models.PaginationDefaults{
	Limit:  50,
	SortBy: models.SortingFieldCreatedAt,
	Order:  models.SortingOrderDesc,
}

type ListTagsParams struct {
	types.PaginationParams
	Target string `form:"target" binding:"omitempty,oneof=case object"`
}

func HandleListTags(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var p ListTagsParams
		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		target := models.TagTargetFromString(p.Target)
		pagination := p.PaginationParams.ToModel(tagPaginationDefaults)

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		tagUsecase := uc.NewTagUseCase()

		tags, hasNextPage, err := tagUsecase.ListTagsPaginated(ctx, orgId, target, pagination)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		nextPageId := ""
		if len(tags) > 0 {
			nextPageId = tags[len(tags)-1].Id
		}

		types.
			NewResponse(pure_utils.Map(tags, dto.AdaptTag)).
			WithPagination(hasNextPage, nextPageId).
			Serve(c)
	}
}

type AddCaseTagsParams struct {
	TagIds []string `json:"tag_ids" binding:"required,min=1,dive,uuid"`
}

func HandleAddCaseTags(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := types.UuidParam(c, "caseId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params AddCaseTagsParams
		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()
		userUsecase := uc.NewUserUseCase()
		tagUsecase := uc.NewTagUseCase()

		cas, err := caseUsecase.AddCaseTags(ctx, caseId.String(), params.TagIds)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		users, err := userUsecase.ListUsers(ctx, &orgId)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		tags, err := tagUsecase.ListAllTags(ctx, orgId, models.TagTargetCase, false)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		referents, err := caseUsecase.GetCasesReferents(ctx, []string{cas.Id})
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptCase(users, tags, referents)(cas)).Serve(c)
	}
}

func HandleRemoveCaseTag(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		caseId, err := types.UuidParam(c, "caseId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		tagId, err := types.UuidParam(c, "tagId")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc)
		caseUsecase := uc.NewCaseUseCase()

		if err := caseUsecase.RemoveCaseTag(ctx, caseId.String(), tagId.String()); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
