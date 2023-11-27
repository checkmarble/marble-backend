package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

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

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	cases, err := usecase.ListCases(organizationId, filters)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, utils.Map(cases, dto.AdaptCaseDto))
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
	c, err := usecase.GetCase(caseInput.Id)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	ctx.JSON(http.StatusOK, dto.AdaptCaseDto(c))
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
		"case": dto.AdaptCaseDto(c),
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
	c, err := usecase.UpdateCase(ctx, userId, models.UpdateCaseAttributes{
		Id:          caseInput.Id,
		Name:        data.Name,
		DecisionIds: data.DecisionIds,
		Status:      models.CaseStatus(data.Status),
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"case": dto.AdaptCaseDto(c),
	})
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
		"case": dto.AdaptCaseDto(c),
	})
}
