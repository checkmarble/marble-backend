package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListGraphRelations(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewGraphRelationUsecase()
		relations, err := uc.ListGraphRelations(ctx)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(relations, dto.AdaptGraphRelationDto))
	}
}

func handleCreateGraphRelation(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var body dto.CreateGraphRelationBody

		if err := c.ShouldBindBodyWithJSON(&body); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewGraphRelationUsecase()
		relation, err := uc.CreateGraphRelation(ctx, dto.AdaptCreateGraphRelation(body, organizationId))

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptGraphRelationDto(relation))
	}
}

func handleDeleteGraphRelation(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		relationId, err := uuid.Parse(c.Param("relation_id"))
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewGraphRelationUsecase()
		if presentError(ctx, c, uc.DeleteGraphRelation(ctx, relationId)) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
