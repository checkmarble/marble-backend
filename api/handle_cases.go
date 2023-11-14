package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleListCases(ctx *gin.Context) {
	organizationId, err := utils.OrgIDFromCtx(ctx.Request.Context(), ctx.Request)
	if presentError(ctx.Writer, ctx.Request, err) {
		return
	}

	// var filters dto.CaseFilters
	// if err := c.ShouldBind(&filters); err != nil {
	// 	c.Status(http.StatusBadRequest)
	// 	return
	// }

	usecase := api.UsecasesWithCreds(ctx.Request).NewCaseUseCase()
	cases, err := usecase.ListCases(organizationId)
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
