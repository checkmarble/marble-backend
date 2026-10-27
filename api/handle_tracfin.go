package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const tracfinRequestBytes = 1 << 20

type tracfinHTTPUsecase interface {
	GetConfiguration(context.Context, uuid.UUID) (models.TracfinConfiguration, error)
	SaveConfiguration(context.Context, uuid.UUID, models.TracfinConfigurationInput) (models.TracfinConfiguration, error)
	GetContext(context.Context, string) (usecases.TracfinContext, error)
	Navigate(context.Context, string, models.TracfinSelection, string, *string, int) (usecases.TracfinNavigationPage, error)
	GetDraft(context.Context, string) (models.TracfinDraft, error)
	CreateDraft(context.Context, string, bool) (models.TracfinDraft, error)
	UpdateDraft(context.Context, string, models.TracfinDraftInput) (models.TracfinDraft, error)
	PrefillAtRevision(context.Context, string, int64) (tracfin.PrefillResult, error)
	ValidateDraft(context.Context, string, int64) (models.TracfinValidation, error)
	Export(context.Context, string, int64) (models.TracfinExport, error)
	DownloadExport(context.Context, string, string) (models.Blob, models.TracfinExport, error)
}

func tracfinError(c *gin.Context, err error) bool {
	if errors.Is(err, models.TracfinSizeLimitError) {
		c.JSON(http.StatusRequestEntityTooLarge, dto.APIErrorResponse{Message: err.Error()})
		return true
	}
	return presentError(c.Request.Context(), c, err)
}
func tracfinBad(c *gin.Context) bool { return tracfinError(c, models.BadParameterError) }
func tracfinID(c *gin.Context, name string) bool {
	_, err := uuid.Parse(c.Param(name))
	if err != nil {
		tracfinBad(c)
		return false
	}
	return true
}
func tracfinJSON(c *gin.Context, target any) bool {
	// Read the bounded body before decoding so an oversized unknown member still returns 413.
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, tracfinRequestBytes))
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			c.JSON(413, dto.APIErrorResponse{Message: "TRACFIN request exceeds 1 MiB"})
		} else {
			tracfinBad(c)
		}
		return false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		tracfinBad(c)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		tracfinBad(c)
		return false
	}
	return true
}

func addTracfinRoutes(router gin.IRoutes, factory func(context.Context) tracfinHTTPUsecase) {
	router.GET("/tracfin/catalog", func(c *gin.Context) {
		catalog, err := tracfin.LoadCatalog()
		if tracfinError(c, err) {
			return
		}
		c.JSON(200, gin.H{"profile": catalog, "limits": gin.H{"request_bytes": tracfinRequestBytes, "navigation_default": 25, "navigation_max": 100, "attachment_bytes": tracfin.MaxAttachmentBytes, "xml_bytes": tracfin.MaxXMLBytes}})
	})
	router.GET("/organizations/:organization_id/tracfin", func(c *gin.Context) {
		if !tracfinID(c, "organization_id") {
			return
		}
		org := uuid.MustParse(c.Param("organization_id"))
		result, err := factory(c.Request.Context()).GetConfiguration(c.Request.Context(), org)
		if !tracfinError(c, err) {
			c.JSON(200, result)
		}
	})
	router.PUT("/organizations/:organization_id/tracfin", func(c *gin.Context) {
		if !tracfinID(c, "organization_id") {
			return
		}
		var r dto.TracfinConfigurationRequest
		if !tracfinJSON(c, &r) {
			return
		}
		if r.ExpectedRevision == nil || *r.ExpectedRevision < 0 {
			tracfinBad(c)
			return
		}
		result, err := factory(c.Request.Context()).SaveConfiguration(c.Request.Context(), uuid.MustParse(c.Param("organization_id")), models.TracfinConfigurationInput{ExpectedRevision: *r.ExpectedRevision, Settings: r.Settings, Mappings: r.Mappings})
		if !tracfinError(c, err) {
			c.JSON(200, result)
		}
	})
	prefix := "/cases/:case_id/tracfin"
	router.GET(prefix+"/context", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		result, err := factory(c.Request.Context()).GetContext(c.Request.Context(), c.Param("case_id"))
		if !tracfinError(c, err) {
			c.JSON(200, result)
		}
	})
	router.GET(prefix+"/draft", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		result, err := factory(c.Request.Context()).GetDraft(c.Request.Context(), c.Param("case_id"))
		if !tracfinError(c, err) {
			c.JSON(200, dto.AdaptTracfinDraft(result))
		}
	})
	router.POST(prefix+"/draft", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		var r dto.TracfinCreateDraftRequest
		if !tracfinJSON(c, &r) {
			return
		}
		if r.DeclarantIsActor == nil {
			tracfinBad(c)
			return
		}
		result, err := factory(c.Request.Context()).CreateDraft(c.Request.Context(), c.Param("case_id"), *r.DeclarantIsActor)
		if !tracfinError(c, err) {
			c.JSON(200, dto.AdaptTracfinDraft(result))
		}
	})
	router.PATCH(prefix+"/draft", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		var r dto.TracfinDraftRequest
		if !tracfinJSON(c, &r) {
			return
		}
		if r.ExpectedRevision == nil || *r.ExpectedRevision < 1 || r.Changes == nil {
			tracfinBad(c)
			return
		}
		result, err := factory(c.Request.Context()).UpdateDraft(c.Request.Context(), c.Param("case_id"), models.TracfinDraftInput{ExpectedRevision: *r.ExpectedRevision, Changes: r.Changes})
		if !tracfinError(c, err) {
			c.JSON(200, dto.AdaptTracfinDraft(result))
		}
	})
	router.POST(prefix+"/navigate", func(c *gin.Context) {
		if !tracfinID(c, "case_id") {
			return
		}
		var r dto.TracfinNavigateRequest
		if !tracfinJSON(c, &r) {
			return
		}
		limit := 25
		if r.Limit != nil {
			limit = *r.Limit
		}
		if limit < 1 || limit > 100 || r.StepID == "" || (r.Cursor != nil && (*r.Cursor == "" || len(*r.Cursor) > 4096)) {
			tracfinBad(c)
			return
		}
		result, err := factory(c.Request.Context()).Navigate(c.Request.Context(), c.Param("case_id"), r.Selection, r.StepID, r.Cursor, limit)
		if !tracfinError(c, err) {
			c.JSON(200, result)
		}
	})
	for _, action := range []string{"prefill", "validate", "export"} {
		router.POST(prefix+"/"+action, func(c *gin.Context) {
			if !tracfinID(c, "case_id") {
				return
			}
			var r dto.TracfinRevisionRequest
			if !tracfinJSON(c, &r) {
				return
			}
			if r.ExpectedRevision == nil || *r.ExpectedRevision < 1 {
				tracfinBad(c)
				return
			}
			uc := factory(c.Request.Context())
			ctx := c.Request.Context()
			id := c.Param("case_id")
			var result any
			var err error
			switch action {
			case "prefill":
				result, err = uc.PrefillAtRevision(ctx, id, *r.ExpectedRevision)
			case "validate":
				result, err = uc.ValidateDraft(ctx, id, *r.ExpectedRevision)
			case "export":
				var e models.TracfinExport
				e, err = uc.Export(ctx, id, *r.ExpectedRevision)
				result = dto.AdaptTracfinExport(e, id)
			}
			if !tracfinError(c, err) {
				c.JSON(200, result)
			}
		})
	}
	router.GET(prefix+"/exports/:export_id", func(c *gin.Context) {
		if !tracfinID(c, "case_id") || !tracfinID(c, "export_id") {
			return
		}
		blob, export, err := factory(c.Request.Context()).DownloadExport(c.Request.Context(), c.Param("case_id"), c.Param("export_id"))
		if blob.ReadCloser != nil {
			defer blob.ReadCloser.Close()
		}
		if tracfinError(c, err) {
			return
		}
		if blob.ReadCloser == nil {
			tracfinError(c, fmt.Errorf("TRACFIN artifact reader unavailable"))
			return
		}
		c.Header("Content-Type", "application/xml; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="tracfin-`+c.Param("export_id")+`.xml"`)
		c.Header("Cache-Control", "no-store")
		c.Header("X-Tracfin-Draft-Revision", strconv.FormatInt(export.DraftRevision, 10))
		c.Status(200)
		if _, err = io.Copy(c.Writer, blob.ReadCloser); err != nil {
			_ = c.Error(err)
		}
	})
}
