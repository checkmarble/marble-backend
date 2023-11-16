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

type Case struct {
	Id string `uri:"case_id" binding:"required,uuid"`
}

func (api *API) handleGetCase(ctx *gin.Context) {
	var caseInput Case
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
		Name:           data.Name,
		Description:    data.Description,
		OrganizationId: organizationId,
		DecisionIds:    data.DecisionIds,
	})

	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"case": dto.AdaptCaseDto(c),
	})
}
