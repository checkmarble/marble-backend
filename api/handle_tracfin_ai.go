package api

import (
	"context"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/gin-gonic/gin"
)

type tracfinAIHTTPUsecase interface {
	ReworkAI(context.Context, string, string, models.TracfinAIInput) (models.TracfinAIRun, error)
	PrepareAI(context.Context, string, int64) (models.TracfinAIRun, error)
	GetAIRun(context.Context, string, string) (models.TracfinAIRun, error)
	ListAIHistory(context.Context, string) ([]models.TracfinAIRunSummary, error)
	SubmitAIAnalysis(context.Context, string, string, models.TracfinAIInput) (models.TracfinDraft, error)
	StartAI(context.Context, string, string, int64) (models.TracfinAIRun, error)
	ResumeAI(context.Context, string, string, int64) (models.TracfinAIRun, error)
	ApproveAI(context.Context, string, string, int64) (models.TracfinDraft, error)
}

func addTracfinAIRoutes(router gin.IRoutes, factory func(context.Context) tracfinAIHTTPUsecase) {
	prefix := "/cases/:case_id/tracfin/ai"
	router.POST(prefix+"/prepare", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		var input models.TracfinAIInput
		if !tracfinJSON(c, &input) {
			return
		}
		r, e := factory(c.Request.Context()).PrepareAI(c.Request.Context(), c.Param("case_id"), input.ExpectedRevision)
		if !tracfinError(c, e) {
			r.PromptSnapshot = nil
			c.JSON(201, r)
		}
	})
	router.GET(prefix+"/history", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		h, e := factory(c.Request.Context()).ListAIHistory(c.Request.Context(), c.Param("case_id"))
		if !tracfinError(c, e) {
			c.JSON(200, gin.H{"runs": h})
		}
	})
	router.GET(prefix+"/runs/:run_id", func(c *gin.Context) {
		if !tracfinID(c, "case_id") || !tracfinID(c, "run_id") {
			return
		}
		r, e := factory(c.Request.Context()).GetAIRun(c.Request.Context(), c.Param("case_id"), c.Param("run_id"))
		if !tracfinError(c, e) {
			r.PromptSnapshot = nil
			c.JSON(200, r)
		}
	})
	for _, action := range []string{"analysis", "start", "resume", "approve", "rework"} {
		router.POST(prefix+"/runs/:run_id/"+action, func(c *gin.Context) {
			if !tracfinID(c, "case_id") || !tracfinID(c, "run_id") {
				return
			}
			var input models.TracfinAIInput
			if !tracfinJSON(c, &input) {
				return
			}
			uc := factory(c.Request.Context())
			ctx := c.Request.Context()
			caseID := c.Param("case_id")
			id := c.Param("run_id")
			if action == "analysis" || action == "approve" {
				var d models.TracfinDraft
				var e error
				if action == "analysis" {
					d, e = uc.SubmitAIAnalysis(ctx, caseID, id, input)
				} else {
					d, e = uc.ApproveAI(ctx, caseID, id, input.ExpectedRevision)
				}
				if !tracfinError(c, e) {
					c.JSON(200, dto.AdaptTracfinDraft(d))
				}
				return
			}
			var r models.TracfinAIRun
			var e error
			if action == "rework" {
				r, e = uc.ReworkAI(ctx, caseID, id, input)
			} else if action == "start" {
				r, e = uc.StartAI(ctx, caseID, id, input.ExpectedRevision)
			} else {
				r, e = uc.ResumeAI(ctx, caseID, id, input.ExpectedRevision)
			}
			if !tracfinError(c, e) {
				r.PromptSnapshot = nil
				c.JSON(202, r)
			}
		})
	}
}
