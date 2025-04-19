package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleGetIngestedObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object_type")
		objectId := c.Param("object_id")

		usecase := usecasesWithCreds(ctx, uc).NewIngestedDataReaderUsecase()
		objects, err := usecase.GetIngestedObject(ctx, organizationID, nil, objectType, objectId, "object_id")
		if presentError(ctx, c, err) {
			return
		}

		if len(objects) == 0 {
			c.JSON(http.StatusNotFound, nil)
			return
		}

		c.JSON(http.StatusOK, objects[0])
	}
}

func handleReadClientDataAsList(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object_type")
		var input dto.ClientDataListRequestBody
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewIngestedDataReaderUsecase()

		// TODO: use adapter
		clientObjects, nextPagination, err := usecase.ReadIngestedClientObjects(ctx, orgId,
			objectType, dto.AdaptClientDataListRequestBody(input))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.ClientDataListResponse{
			Data:       clientObjects,
			Pagination: dto.AdaptClientDataListPaginationDto(nextPagination),
		})
	}
}
