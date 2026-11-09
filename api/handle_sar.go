package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/sar"
	"github.com/gin-gonic/gin"
)

type sarHTTPUsecase interface {
	ListFormats(context.Context) ([]usecases.SarFormatInfo, error)
	GetConfiguration(context.Context) (models.SarConfiguration, error)
	PutConfiguration(context.Context, usecases.SarConfigurationInput) (models.SarConfiguration, error)
	ListMemory(context.Context, string) ([]models.SarMemoryEntry, error)
	GetPivot(context.Context, string) (usecases.SarPivotView, error)
	PatchPivot(context.Context, string, usecases.SarPatchInput) (usecases.SarPivotView, error)
	ValidatePivotFacts(context.Context, string, int64) (usecases.SarPivotView, error)
	ListRevisions(context.Context, string, string) ([]models.SarRevision, error)
	StartRun(context.Context, string, usecases.SarRunInput) (models.SarRun, error)
	GetRun(context.Context, string, string) (models.SarRun, error)
	ListReports(context.Context, string) ([]models.SarReport, error)
	GetReportView(context.Context, string, string) (sar.ReportView, error)
	PatchReport(context.Context, string, string, usecases.SarReportPatchInput) (sar.ReportView, error)
	Approve(context.Context, string, string, int64) (sar.ReportView, error)
	Export(context.Context, string, string) (sar.Artifact, error)
	ListChat(context.Context, string, string) ([]models.SarChatMessage, error)
	PostChat(context.Context, string, string, usecases.SarChatInput) (usecases.SarChatResult, error)
	ApplyChat(context.Context, string, string, string) (usecases.SarChatResult, error)
}

type sarRevisionInput struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

func addSarRoutes(router gin.IRoutes, factory func(context.Context) sarHTTPUsecase) {
	uc := func(c *gin.Context) sarHTTPUsecase { return factory(c.Request.Context()) }
	ok := func(c *gin.Context, status int, v any, err error) {
		if !tracfinError(c, err) {
			c.JSON(status, v)
		}
	}
	ids := func(c *gin.Context, names ...string) bool {
		for _, n := range names {
			if !tracfinID(c, n) {
				return false
			}
		}
		return true
	}
	router.GET("/sar/formats", func(c *gin.Context) {
		v, err := uc(c).ListFormats(c.Request.Context())
		ok(c, http.StatusOK, gin.H{"formats": v}, err)
	})
	router.GET("/sar/configuration", func(c *gin.Context) {
		v, err := uc(c).GetConfiguration(c.Request.Context())
		ok(c, http.StatusOK, v, err)
	})
	router.PUT("/sar/configuration", func(c *gin.Context) {
		var in usecases.SarConfigurationInput
		if !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).PutConfiguration(c.Request.Context(), in)
		ok(c, http.StatusOK, v, err)
	})
	router.GET("/sar/memory", func(c *gin.Context) {
		v, err := uc(c).ListMemory(c.Request.Context(), c.Query("format_key"))
		ok(c, http.StatusOK, gin.H{"entries": v}, err)
	})
	prefix := "/cases/:case_id/sar"
	router.GET(prefix+"/pivot", func(c *gin.Context) {
		if !ids(c, "case_id") {
			return
		}
		v, err := uc(c).GetPivot(c.Request.Context(), c.Param("case_id"))
		ok(c, http.StatusOK, v, err)
	})
	router.PATCH(prefix+"/pivot", func(c *gin.Context) {
		var in usecases.SarPatchInput
		if !ids(c, "case_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).PatchPivot(c.Request.Context(), c.Param("case_id"), in)
		ok(c, http.StatusOK, v, err)
	})
	router.POST(prefix+"/pivot/validate", func(c *gin.Context) {
		var in sarRevisionInput
		if !ids(c, "case_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).ValidatePivotFacts(c.Request.Context(), c.Param("case_id"), in.ExpectedRevision)
		ok(c, http.StatusOK, v, err)
	})
	router.GET(prefix+"/revisions", func(c *gin.Context) {
		if !ids(c, "case_id") {
			return
		}
		v, err := uc(c).ListRevisions(c.Request.Context(), c.Param("case_id"), c.Query("object"))
		ok(c, http.StatusOK, gin.H{"revisions": v}, err)
	})
	router.POST(prefix+"/runs", func(c *gin.Context) {
		var in usecases.SarRunInput
		if !ids(c, "case_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).StartRun(c.Request.Context(), c.Param("case_id"), in)
		ok(c, http.StatusAccepted, v, err)
	})
	router.GET(prefix+"/runs/:run_id", func(c *gin.Context) {
		if !ids(c, "case_id", "run_id") {
			return
		}
		v, err := uc(c).GetRun(c.Request.Context(), c.Param("case_id"), c.Param("run_id"))
		ok(c, http.StatusOK, v, err)
	})
	router.GET(prefix+"/reports", func(c *gin.Context) {
		if !ids(c, "case_id") {
			return
		}
		v, err := uc(c).ListReports(c.Request.Context(), c.Param("case_id"))
		ok(c, http.StatusOK, gin.H{"reports": v}, err)
	})
	rp := prefix + "/reports/:report_id"
	router.GET(rp, func(c *gin.Context) {
		if !ids(c, "case_id", "report_id") {
			return
		}
		v, err := uc(c).GetReportView(c.Request.Context(), c.Param("case_id"), c.Param("report_id"))
		ok(c, http.StatusOK, v, err)
	})
	router.PATCH(rp, func(c *gin.Context) {
		var in usecases.SarReportPatchInput
		if !ids(c, "case_id", "report_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).PatchReport(c.Request.Context(), c.Param("case_id"), c.Param("report_id"), in)
		ok(c, http.StatusOK, v, err)
	})
	router.POST(rp+"/approve", func(c *gin.Context) {
		var in sarRevisionInput
		if !ids(c, "case_id", "report_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).Approve(c.Request.Context(), c.Param("case_id"), c.Param("report_id"), in.ExpectedRevision)
		ok(c, http.StatusOK, v, err)
	})
	router.GET(rp+"/export", func(c *gin.Context) {
		if !ids(c, "case_id", "report_id") {
			return
		}
		a, err := uc(c).Export(c.Request.Context(), c.Param("case_id"), c.Param("report_id"))
		if tracfinError(c, err) {
			return
		}
		c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(a.Name))
		c.Data(http.StatusOK, a.Mime, a.Data)
	})
	router.GET(rp+"/chat", func(c *gin.Context) {
		if !ids(c, "case_id", "report_id") {
			return
		}
		v, err := uc(c).ListChat(c.Request.Context(), c.Param("case_id"), c.Param("report_id"))
		ok(c, http.StatusOK, gin.H{"messages": v}, err)
	})
	router.POST(rp+"/chat", func(c *gin.Context) {
		var in usecases.SarChatInput
		if !ids(c, "case_id", "report_id") || !tracfinJSON(c, &in) {
			return
		}
		v, err := uc(c).PostChat(c.Request.Context(), c.Param("case_id"), c.Param("report_id"), in)
		ok(c, http.StatusCreated, v, err)
	})
	router.POST(rp+"/chat/:message_id/apply", func(c *gin.Context) {
		if !ids(c, "case_id", "report_id", "message_id") {
			return
		}
		v, err := uc(c).ApplyChat(c.Request.Context(), c.Param("case_id"), c.Param("report_id"), c.Param("message_id"))
		ok(c, http.StatusOK, v, err)
	})
}
