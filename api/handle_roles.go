package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleGetRoles(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		userUsecase := uc.NewUserUseCase()

		roles, permissions, err := userUsecase.GetRoles(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"roles":       roles,
			"permissions": permissions,
		})
	}
}
