package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleCreateDataModelPivot(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreatePivotInput
		if err := c.ShouldBind(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		pivot, err := usecase.CreatePivot(
			c.Request.Context(),
			dto.AdaptCreatePivotInput(input, organizationID),
		)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"pivot": dto.AdaptPivotDto(pivot),
		})
	}
}

func handleListDataModelPivots(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filters struct {
			TableId *string `form:"table_id"`
		}
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		pivots, err := usecase.ListPivots(c.Request.Context(), organizationID, filters.TableId)
		if presentError(ctx, c, err) {
			return
		}

		pivotsDto := pure_utils.Map(pivots, dto.AdaptPivotDto)
		c.JSON(http.StatusOK, gin.H{
			"pivots": pivotsDto,
		})
	}
}
