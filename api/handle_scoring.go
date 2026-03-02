package api

import (
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/scoring"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleScoringGetSettings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringSettingsUsecase()
		settings, err := scoringUsecase.GetSettings(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptSettings(settings))
	}
}

func handleScoringUpdateSettings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload scoring.UpdateSettingsRequest

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		req := models.ScoringSettings{
			MaxRiskLevel: payload.MaxRiskLevel,
		}

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringSettingsUsecase()
		settings, err := scoringUsecase.UpdateSettings(ctx, req)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptSettings(settings))
	}
}

func handleScoringComputeScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()
		_, eval, err := scoringUsecase.ComputeScore(ctx, c.Param("recordType"), c.Param("recordId"))
		if presentError(ctx, c, err) {
			return
		}
		if eval == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptScoringEvaluation(*eval))
	}
}

func handleScoringListRulesets(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringRulesetsUsecase()

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
		scoringUsecase := uc.NewScoringRulesetsUsecase()

		var status models.ScoreRulesetStatus

		switch c.Query("status") {
		case "draft":
			status = models.ScoreRulesetDraft
		case "committed", "":
			status = models.ScoreRulesetCommitted
		default:
			presentError(ctx, c, errors.Wrapf(models.BadParameterError, "unknown status '%s'", c.Query("status")))
			return
		}

		ruleset, err := scoringUsecase.GetRuleset(ctx, c.Param("recordType"), status)
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

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringRulesetsUsecase()

		ruleset, err := scoringUsecase.CreateRulesetVersion(ctx, c.Param("recordType"), payload)
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

func handleScoringGetRulesetPreparationStatus(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringRulesetsUsecase()

		status, err := scoringUsecase.PreparationStatus(ctx, c.Param("recordType"))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptPublicationPreparationStatus(status))
	}
}

func handleScoringPrepareRuleset(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringRulesetsUsecase()

		err := scoringUsecase.PrepareRuleset(ctx, c.Param("recordType"))
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleScoringCommitRuleset(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringRulesetsUsecase()

		ruleset, err := scoringUsecase.CommitRuleset(ctx, c.Param("recordType"))
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

func handleScoringScoreHistory(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		record := models.ScoringRecordRef{
			RecordType: c.Param("recordType"),
			RecordId:   c.Param("recordId"),
		}

		scores, err := scoringUsecase.GetScoreHistory(ctx, record)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(scores, scoring.AdaptScore(nil)))
	}
}

func handleScoringGetActiveScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		record := models.ScoringRecordRef{
			RecordType: c.Param("recordType"),
			RecordId:   c.Param("recordId"),
		}

		opts := models.RefreshScoreOptions{
			RefreshOlderThan:    time.Hour,
			RefreshInBackground: false,
		}

		score, evals, err := scoringUsecase.GetActiveScore(ctx, record, c.Query("include_evaluation") == "true", opts)
		if presentError(ctx, c, err) {
			return
		}

		if score == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, scoring.AdaptScore(evals)(*score))
	}
}

func handleOverrideRecordScore(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload scoring.OverrideScoreRequest

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		req := models.InsertScoreRequest{
			RecordType: c.Param("recordType"),
			RecordId:   c.Param("recordId"),
			RiskLevel:  payload.RiskLevel,
			Source:     models.ScoreSourceOverride,
			StaleAt:    payload.StaleAt,
		}

		score, err := scoringUsecase.OverrideScore(ctx, req)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, scoring.AdaptScore(nil)(score))
	}
}

func handleScoringGetDistribution(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		uc := usecasesWithCreds(ctx, uc)
		scoringUsecase := uc.NewScoringScoresUsecase()

		scores, err := scoringUsecase.GetScoreDistribution(ctx, c.Param("entityType"))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(scores, scoring.AdaptScoreDistribution))
	}
}
