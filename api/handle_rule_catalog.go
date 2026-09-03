package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func getRuleCatalog(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		catalogUsecase := uc.NewRuleCatalogUsecase()

		catalog, err := catalogUsecase.GetRuleCatalog()
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, catalog)
	}
}
