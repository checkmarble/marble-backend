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

	usecase := api.UsecasesWithCreds(c.Request).NewCaseUseCase()
	cases, err := usecase.ListCases(organizationId)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	c.JSON(http.StatusOK, utils.Map(cases, dto.AdaptCaseDto))
}
