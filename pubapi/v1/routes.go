package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases) {
	r.GET("/-/version", handleVersion)

	{
		r := r.Group("/", authMiddleware)

		r.POST("/example", handleExampleValidation)

		r.GET("/decisions/:decisionId/sanction-checks", HandleListSanctionChecks(uc))

		r.POST("/sanction-checks/:sanctionCheckId/refine", HandleRefineSanctionCheck(uc, true))
		r.POST("/sanction-checks/:sanctionCheckId/search", HandleRefineSanctionCheck(uc, false))
		r.POST("/sanction-checks/search", HandleSanctionFreeformSearch(uc))

		r.GET("/sanction-checks/entities/:entityId", HandleGetSanctionCheckEntity(uc))
		r.POST("/sanction-checks/matches/:matchId",
			HandleUpdateSanctionCheckMatchStatus(uc))

		r.POST("/sanction-checks/whitelists/search", HandleSearchWhitelist(uc))
		r.POST("/sanction-checks/whitelists", HandleAddWhitelist(uc))
		r.DELETE("/sanction-checks/whitelists", HandleDeleteWhitelist(uc))
	}
}

func handleVersion(c *gin.Context) {
	pubapi.NewResponse(gin.H{"version": "v1a"}).Serve(c)
}

type ExamplePayload struct {
	Age    int    `json:"age" binding:"required,gt=18"`
	Email  string `json:"email" binding:"required,email"`
	IsNice string `json:"is_nice" binding:"required,boolean"`
}

func handleExampleValidation(c *gin.Context) {
	var payload ExamplePayload

	if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
		pubapi.NewErrorResponse().WithError(err).Serve(c)
		return
	}

	c.Status(http.StatusOK)
}
