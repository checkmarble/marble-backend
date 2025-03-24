package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

type SnoozeRuleParams struct {
	RuleId   string `json:"rule_id" binding:"required,uuid"`
	Duration string `json:"duration" binding:"required"`
}

func HandleSnoozeRule(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		decisionId, err := pubapi.UuidParam(c, "decisionId")
		if err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params SnoozeRuleParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		creds, _ := utils.CredentialsFromCtx(c.Request.Context())
		uc := pubapi.UsecasesWithCreds(c.Request.Context(), uc)
		ruleSnoozeUsecase := uc.NewRuleSnoozeUsecase()

		snooze := models.SnoozeDecisionInput{
			OrganizationId: creds.OrganizationId,
			DecisionId:     decisionId.String(),
			RuleId:         params.RuleId,
			Duration:       params.Duration,
		}

		if _, err = ruleSnoozeUsecase.SnoozeDecisionWithoutCase(c.Request.Context(), snooze); err != nil {
			pubapi.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusCreated)
	}
}
