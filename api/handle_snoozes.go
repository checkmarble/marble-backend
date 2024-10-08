package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleSnoozesOfDecision(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		_, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		decisionId := c.Param("decision_id")
		_, err = uuid.Parse(decisionId)
		if err != nil {
			presentError(c, errors.Wrap(models.BadParameterError, "decision_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(c.Request, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.ActiveSnoozesForDecision(c.Request.Context(), decisionId)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"snoozes": dto.AdaptSnoozesOfDecision(snoozes)})
	}
}

func handleSnoozeDecision(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(c.Request.Context())
		userId := creds.ActorIdentity.UserId

		decisionId := c.Param("decision_id")
		_, err = uuid.Parse(decisionId)
		if err != nil {
			presentError(c, errors.Wrap(models.BadParameterError, "decision_id must be a valid uuid"))
			return
		}

		var input dto.SnoozeDecisionInput
		if presentError(c, c.BindJSON(&input)) {
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(c.Request, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.SnoozeDecision(c.Request.Context(), models.SnoozeDecisionInput{
			Comment:        input.Comment,
			DecisionId:     decisionId,
			Duration:       input.Duration,
			OrganizationId: organizationId,
			RuleId:         input.RuleId,
			UserId:         userId,
		})
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"snoozes": dto.AdaptSnoozesOfDecision(snoozes)})
	}
}

func handleSnoozesOfScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		_, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		scenarioIterationId := c.Param("iteration_id")
		_, err = uuid.Parse(scenarioIterationId)
		if err != nil {
			presentError(c, errors.Wrap(models.BadParameterError,
				"scenario_iteration_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(c.Request, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.ActiveSnoozesForScenarioIteration(
			c.Request.Context(), scenarioIterationId)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"snoozes": dto.AdaptSnoozesOfIteration(snoozes)})
	}
}

func handleGetSnoozesById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ruleSnoozeId := c.Param("rule_snooze_id")
		_, err := uuid.Parse(ruleSnoozeId)
		if err != nil {
			presentError(c, errors.Wrap(models.BadParameterError,
				"rule_snooze_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(c.Request, uc).NewRuleSnoozeUsecase()
		snooze, err := ruleSnoozeUsecase.GetRuleSnoozeById(c.Request.Context(), ruleSnoozeId)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"snooze": dto.AdaptRuleSnoose(snooze)})
	}
}
