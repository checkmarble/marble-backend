package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func (api *API) createDataModelPivot(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	var input dto.CreatePivotInput
	if err := c.ShouldBind(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	pivot, err := usecase.CreatePivot(c.Request.Context(), organizationID, dto.AdaptCreatePivotInput(input))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pivot": dto.AdaptPivotDto(pivot),
	})
}

func (api *API) listDataModelPivots(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	var filters struct {
		TableId *string `form:"table_id"`
	}
	if err := c.ShouldBind(&filters); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	pivots, err := usecase.ListPivots(c.Request.Context(), organizationID, filters.TableId)
	if presentError(c, err) {
		return
	}

	pivotsDto := pure_utils.Map(pivots, dto.AdaptPivotDto)
	c.JSON(http.StatusOK, gin.H{
		"pivots": pivotsDto,
	})
}
