package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleScoringComputeScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()
		eval, err := scoringUsecase.ComputeScore(ctx, c.Param("entityType"), c.Param("entityId"))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptScoringEvaluation(eval))
	}
}

func handleScoringListRulesets(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		rulesets, err := scoringUsecase.ListRulesets(ctx)
		if presentError(ctx, c, err) {
			return
		}

		out, err := pure_utils.MapErr(rulesets, scoring.AdaptScoringRuleset)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleScoringGetRuleset(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		ruleset, err := scoringUsecase.GetRuleset(ctx, c.Param("entityType"))
		if presentError(ctx, c, err) {
			return
		}

		out, err := scoring.AdaptScoringRuleset(ruleset)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleScoringCreateRulesetVersion(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload scoring.CreateRulesetRequest

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		ruleset, err := scoringUsecase.CreateRulesetVersion(ctx, c.Param("entityType"), payload)
		if presentError(ctx, c, err) {
			return
		}

		out, err := scoring.AdaptScoringRuleset(ruleset)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleScoringGetScoreHistory(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		entityRef := models.ScoringEntityRef{
			EntityType: c.Param("entityType"),
			EntityId:   c.Param("entityId"),
		}

		scores, err := scoringUsecase.GetScoreHistory(ctx, entityRef)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(scores, scoring.AdaptScore))
	}
}

func handleScoringGetActiveScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		entityRef := models.ScoringEntityRef{
			EntityType: c.Param("entityType"),
			EntityId:   c.Param("entityId"),
		}

		score, err := scoringUsecase.GetActiveScore(ctx, entityRef)
		if presentError(ctx, c, err) {
			return
		}

		if score == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptScore(*score))
	}
}

func handleOverrideEntityScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload scoring.OverrideScoreRequest

		if err := c.ShouldBindBodyWithJSON(&payload); presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringUsecase()

		req := models.InsertScoreRequest{
			EntityType: c.Param("entityType"),
			EntityId:   c.Param("entityId"),
			Score:      payload.Score,
			Source:     models.ScoreSourceOverride,
			StaleAt:    payload.StaleAt,
		}

		score, err := scoringUsecase.OverrideScore(ctx, req)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, scoring.AdaptScore(score))
	}
}
