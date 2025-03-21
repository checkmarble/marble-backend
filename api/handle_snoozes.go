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
		ctx := c.Request.Context()
		_, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		decisionId := c.Param("decision_id")
		_, err = uuid.Parse(decisionId)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "decision_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(ctx, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.ActiveSnoozesForDecision(ctx, decisionId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"snoozes": dto.AdaptSnoozesOfDecision(snoozes)})
	}
}

func handleSnoozeDecision(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(ctx)
		userId := creds.ActorIdentity.UserId

		decisionId := c.Param("decision_id")
		_, err = uuid.Parse(decisionId)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "decision_id must be a valid uuid"))
			return
		}

		var input dto.SnoozeDecisionInput
		if presentError(ctx, c, c.BindJSON(&input)) {
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(ctx, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.SnoozeDecision(ctx, models.SnoozeDecisionInput{
			Comment:        input.Comment,
			DecisionId:     decisionId,
			Duration:       input.Duration,
			OrganizationId: organizationId,
			RuleId:         input.RuleId,
			UserId:         &userId,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"snoozes": dto.AdaptSnoozesOfDecision(snoozes)})
	}
}

func handleSnoozesOfScenarioIteration(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		_, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		scenarioIterationId := c.Param("iteration_id")
		_, err = uuid.Parse(scenarioIterationId)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"scenario_iteration_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(ctx, uc).NewRuleSnoozeUsecase()
		snoozes, err := ruleSnoozeUsecase.ActiveSnoozesForScenarioIteration(
			ctx, scenarioIterationId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"snoozes": dto.AdaptSnoozesOfIteration(snoozes)})
	}
}

func handleGetSnoozesById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ruleSnoozeId := c.Param("rule_snooze_id")
		_, err := uuid.Parse(ruleSnoozeId)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"rule_snooze_id must be a valid uuid"))
			return
		}

		ruleSnoozeUsecase := usecasesWithCreds(ctx, uc).NewRuleSnoozeUsecase()
		snooze, err := ruleSnoozeUsecase.GetRuleSnoozeById(ctx, ruleSnoozeId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"snooze": dto.AdaptRuleSnoose(snooze)})
	}
}
