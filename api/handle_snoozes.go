package api

import (
	"net/http"
	"time"

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

	// decisionUsecase := api.UsecasesWithCreds(c.Request).NewDecisionUsecase()
	// decisions, nbSkipped, err := decisionUsecase.CreateAllDecisions(
	// 	c.Request.Context(),
	// 	models.CreateAllDecisionsInput{
	// 		OrganizationId:     organizationId,
	// 		PayloadRaw:         requestData.TriggerObjectRaw,
	// 		TriggerObjectTable: requestData.TriggerObjectType,
	// 	},
	// )
	if presentError(c, err) {
		return
	}
	snoozes := dto.AdaptSnoozesOfDecision(models.NewSnoozesOfDecision(decisionId,
		[]models.RuleSnooze{{
			Id: "1", SnoozeGroupId: "1", PivotValue: "1", StartsAt: time.Now(), EndsAt: time.Now(), CreatedBy: "1",
		}},
		models.ScenarioIteration{
			Rules: []models.Rule{{Id: "1", SnoozeGroupId: "1"}, {Id: "2", SnoozeGroupId: "2"}},
		},
	))
	c.JSON(http.StatusOK, gin.H{"snoozes": snoozes})
}
