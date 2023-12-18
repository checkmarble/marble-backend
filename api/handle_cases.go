package api

import (
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

var casesPaginationDefaults = dto.PaginationDefaults{
	Limit:  25,
	SortBy: models.CasesSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func (api *API) handleListCases(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	var filters dto.CaseFilters
	if err := ctx.ShouldBind(&filters); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var paginationAndSorting dto.PaginationAndSortingInput
	if err := ctx.ShouldBind(&paginationAndSorting); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	paginationAndSorting = dto.WithPaginationDefaults(paginationAndSorting, casesPaginationDefaults)

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	cases, err := usecase.ListCases(ctx.Request.Context(), organizationId, dto.AdaptPaginationAndSortingInput(paginationAndSorting), filters)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	if len(cases) == 0 {
		ctx.JSON(http.StatusOK, gin.H{
			"total":      0,
			"startIndex": 0,
			"endIndex":   0,
			"items":      []dto.APICase{},
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"total":      cases[0].Total,
		"startIndex": cases[0].RankNumber,
		"endIndex":   cases[len(cases)-1].RankNumber,
		"items":      utils.Map(cases, func(c models.CaseWithRank) dto.APICase { return dto.AdaptCaseDto(c.Case) }),
	})
}

type CaseInput struct {
	Id string `uri:"case_id" binding:"required,uuid"`
}

func (api *API) handleGetCase(ctx *gin.Context) {
	var caseInput CaseInput
	if err := ctx.ShouldBindUri(&caseInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	c, err := usecase.GetCase(ctx.Request.Context(), caseInput.Id)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, dto.AdaptCaseWithDecisionsDto(c))
}

func (api *API) handlePostCase(ctx *gin.Context) {
	creds, found := utils.CredentialsFromCtx(ctx.Request.Context())
	if !found {
		presentError(ctx.Writer, ctx.Request, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var data dto.CreateCaseBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	c, err := usecase.CreateCase(ctx, userId, models.CreateCaseAttributes{
		DecisionIds:    data.DecisionIds,
		InboxId:        data.InboxId,
		Name:           data.Name,
		OrganizationId: organizationId,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(c),
	})
}

func (api *API) handlePatchCase(ctx *gin.Context) {
	creds, found := utils.CredentialsFromCtx(ctx.Request.Context())
	if !found {
		presentError(ctx.Writer, ctx.Request, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var caseInput CaseInput
	if err := ctx.ShouldBindUri(&caseInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var data dto.UpdateCaseBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	c, err := usecase.UpdateCase(ctx.Request.Context(), userId, models.UpdateCaseAttributes{
		Id:      caseInput.Id,
		Name:    data.Name,
		Status:  models.CaseStatus(data.Status),
		InboxId: data.InboxId,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(c),
	})
}

func (api *API) handlePostCaseDecisions(ctx *gin.Context) {
	creds, found := utils.CredentialsFromCtx(ctx.Request.Context())
	if !found {
		presentError(ctx.Writer, ctx.Request, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var caseInput CaseInput
	if err := ctx.ShouldBindUri(&caseInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var data dto.AddDecisionToCaseBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	c, err := usecase.AddDecisionsToCase(ctx.Request.Context(), userId, caseInput.Id, data.DecisionIds)

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"case": dto.AdaptCaseWithDecisionsDto(c)})
}

func (api *API) handlePostCaseComment(ctx *gin.Context) {
	creds, found := utils.CredentialsFromCtx(ctx.Request.Context())
	if !found {
		presentError(ctx.Writer, ctx.Request, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var caseInput CaseInput
	if err := ctx.ShouldBindUri(&caseInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var data dto.CreateCaseCommentBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	c, err := usecase.CreateCaseComment(ctx, userId, models.CreateCaseCommentAttributes{
		Id:      caseInput.Id,
		Comment: data.Comment,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"case": dto.AdaptCaseWithDecisionsDto(c),
	})
}

func (api *API) handlePostCaseTags(ctx *gin.Context) {
	creds, found := utils.CredentialsFromCtx(ctx.Request.Context())
	if !found {
		presentError(ctx.Writer, ctx.Request, fmt.Errorf("no credentials in context"))
		return
	}
	userId := string(creds.ActorIdentity.UserId)

	var caseInput CaseInput
	if err := ctx.ShouldBindUri(&caseInput); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var data dto.CreateCaseTagBody
	if err := ctx.ShouldBindJSON(&data); err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	c, err := usecase.CreateCaseTags(ctx.Request.Context(), userId, models.CreateCaseTagsAttributes{
		CaseId: caseInput.Id,
		TagIds: data.TagIds,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(c)})
}

type FileForm struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
}

func (api *API) handlePostCaseFile(c *gin.Context) {
	var caseInput CaseInput
	if err := c.ShouldBindUri(&caseInput); err != nil {
		presentError(c.Writer, c.Request, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	var form FileForm
	if err := c.ShouldBind(&form); err != nil {
		presentError(c.Writer, c.Request, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	cs, err := usecase.CreateCaseFile(c.Request.Context(), models.CreateCaseFileInput{
		CaseId: caseInput.Id,
		File:   form.File,
	})
	if presentError(c.Writer, c.Request, err) {
		return
	}

	c.JSON(http.StatusCreated, gin.H{"case": dto.AdaptCaseWithDecisionsDto(cs)})
}
