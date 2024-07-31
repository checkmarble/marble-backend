package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (api *API) handleSnoozesOfDecision(c *gin.Context) {
	_, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	decisionId := c.Param("decision_id")
	_, err = uuid.Parse(decisionId)
	if err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, "decision_id must be a valid uuid"))
		return
	}

	ruleSnoozeUsecase := api.UsecasesWithCreds(c.Request).NewRuleSnoozeUsecase()
	snoozes, err := ruleSnoozeUsecase.ActiveSnoozesForDecision(c.Request.Context(), decisionId)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"snoozes": dto.AdaptSnoozesOfDecision(snoozes)})
}
