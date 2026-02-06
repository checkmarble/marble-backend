package api

import (
	"net/http"
	"strconv"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleClient360ListTables(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc)
		clientUsecase := uc.NewClient360Usecase()

		tables, err := clientUsecase.ListTables(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(tables, dto.AdaptClient360Table))
	}
}

func handleClient360SearchObjects(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.Client360SearchInput

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			return
		}

		page := 1
		if ps := c.Query("page"); ps != "" {
			p, err := strconv.Atoi(ps)
			if err != nil {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid page number"))
				return
			}
			if p == 0 {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid page number, must be greater than 0"))
				return
			}
			page = p
		}

		input := models.Client360SearchInput{
			Table: payload.Table,
			Terms: payload.Terms,
			Page:  uint64(page),
		}

		uc := usecasesWithCreds(ctx, uc)
		clientUsecase := uc.NewClient360Usecase()

		objects, err := clientUsecase.SearchObject(ctx, input)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.Paginated[map[string]any]{
			Items:       pure_utils.Map(objects.Items, func(c models.DataModelObject) map[string]any { return c.Data }),
			HasNextPage: objects.HasNextPage,
		})
	}
}
