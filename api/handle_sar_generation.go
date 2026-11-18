package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/sargen"
	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
)

// HTTP handlers of the SAR declarations. Rulings: the request DTO maps only path+value to core.Fill; errors go through
// presentError, or presentSarError for writes that can be refused with issues
// (Conflict 409, BadParameter 400, Forbidden 403, NotFound 404, invalid UUID 400); the usecase comes from newSarGenerationUsecase(c, uc).

const (
	sarNavigateDefaultLimit = 100
	sarNavigateMaxLimit     = 100
)

// sarUuidParam parses a UUID path parameter; an invalid one is a 400.
func sarUuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		presentError(c.Request.Context(), c,
			errors.Wrapf(models.BadParameterError, "identifiant invalide pour %s", name))
		return uuid.Nil, false
	}
	return id, true
}

// sarBindJSON binds the body; any failure is a 400.
func sarBindJSON(c *gin.Context, into any) bool {
	err := c.ShouldBindJSON(into)
	if err == nil {
		return true
	}
	var verr validator.ValidationErrors
	if !errors.As(err, &verr) {
		err = errors.Wrap(models.BadParameterError, "corps de requête invalide : "+err.Error())
	}
	presentError(c.Request.Context(), c, err)
	return false
}

// sarIssuesCarrier is a refused write exposing its structured issues.
type sarIssuesCarrier interface {
	error
	SarIssues() []core.Issue
}

// presentSarError is presentError, except that a refusal carrying issues is a
// 400 with details.issues (same shape as PUT /sar-format-settings) and the clean
// message of the refusal.
func presentSarError(c *gin.Context, err error) bool {
	var carrier sarIssuesCarrier
	if err != nil && errors.Is(err, models.BadParameterError) && errors.As(err, &carrier) {
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message: carrier.Error(),
			Details: dto.SarIssuesDetails{Issues: pure_utils.Map(carrier.SarIssues(), dto.AdaptSarIssueDto)},
		})
		return true
	}
	return presentError(c.Request.Context(), c, err)
}

func sarState(c *gin.Context, code int, st sargen.DeclarationState, err error) {
	if presentSarError(c, err) {
		return
	}
	c.JSON(code, adaptSarDeclarationDto(st))
}

func handleListSarFormats(_ usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, pure_utils.Map(formats.List(), dto.AdaptSarFormatDto))
	}
}

func handleGetSarFormatSpec(_ usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, err := core.ParseFormatKey(c.Param("format_key"))
		var f core.Format
		ok := false
		if err == nil {
			f, ok = formats.Get(key)
		}
		if !ok {
			presentError(c.Request.Context(), c, errors.Wrap(models.NotFoundError, "format inconnu"))
			return
		}
		c.JSON(http.StatusOK, dto.AdaptSarSpecDto(f))
	}
}

func handleGetSarOrgSettings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, issues, err := newSarGenerationUsecase(c, uc).GetOrgSettings(c.Request.Context(), c.Param("family"))
		if presentError(c.Request.Context(), c, err) {
			return
		}
		ready, err := newSarGenerationUsecase(c, uc).SettingsReadiness(c.Request.Context(), c.Param("family"), s.Values)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, adaptSarOrgSettingsDto(s, issues, ready))
	}
}

func handlePutSarOrgSettings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in dto.SarOrgSettingsInput
		if !sarBindJSON(c, &in) {
			return
		}
		s, issues, err := newSarGenerationUsecase(c, uc).PutOrgSettings(c.Request.Context(), c.Param("family"), in.Values)
		if err != nil && len(issues) > 0 && errors.Is(err, models.BadParameterError) {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: err.Error(),
				Details: dto.SarIssuesDetails{Issues: pure_utils.Map(issues, dto.AdaptSarIssueDto)},
			})
			return
		}
		if presentError(c.Request.Context(), c, err) {
			return
		}
		ready, err := newSarGenerationUsecase(c, uc).SettingsReadiness(c.Request.Context(), c.Param("family"), s.Values)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, adaptSarOrgSettingsDto(s, issues, ready))
	}
}

func handleCreateSarDeclaration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		caseId, ok := sarUuidParam(c, "case_id")
		if !ok {
			return
		}
		var in dto.SarCreateDeclarationInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).CreateDeclaration(c.Request.Context(), caseId, in.FormatKey, in.ReportType)
		sarState(c, http.StatusCreated, st, err)
	}
}

func handleListSarDeclarations(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		caseId, ok := sarUuidParam(c, "case_id")
		if !ok {
			return
		}
		ds, err := newSarGenerationUsecase(c, uc).ListDeclarations(c.Request.Context(), caseId)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(ds, dto.AdaptSarDeclarationSummaryDto))
	}
}

func handleGetSarDeclaration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).GetDeclaration(c.Request.Context(), id)
		sarState(c, http.StatusOK, st, err)
	}
}

func handleUpdateSarFields(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarUpdateFieldsInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).UpdateFields(c.Request.Context(), id, in.Version, dto.AdaptSarFills(in.Fields))
		sarState(c, http.StatusOK, st, err)
	}
}

func handleAddSarItem(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarAddItemInput
		if !sarBindJSON(c, &in) {
			return
		}
		item, st, err := newSarGenerationUsecase(c, uc).AddItem(c.Request.Context(), id, in.Version, core.Path(in.Container))
		if presentSarError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, dto.SarAddItemResponse{Item: string(item), State: adaptSarDeclarationDto(st)})
	}
}

func handleRemoveSarItem(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarRemoveItemInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).RemoveItem(c.Request.Context(), id, in.Version, core.Path(in.Item))
		sarState(c, http.StatusOK, st, err)
	}
}

func handleMarkSarReviewed(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarMarkReviewedInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).MarkReviewed(c.Request.Context(), id, in.Version, dto.AdaptSarPaths(in.Paths))
		sarState(c, http.StatusOK, st, err)
	}
}

func handleExportSarDeclaration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarVersionInput
		if !sarBindJSON(c, &in) {
			return
		}
		e, err := newSarGenerationUsecase(c, uc).Export(c.Request.Context(), id, in.Version)
		if presentSarError(c, err) {
			return
		}
		c.JSON(http.StatusCreated, dto.AdaptSarExportDto(e))
	}
}

func handleListSarExports(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		es, err := newSarGenerationUsecase(c, uc).ListExports(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(es, dto.AdaptSarExportDto))
	}
}

func handleDownloadSarExport(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		exportId, ok := sarUuidParam(c, "export_id")
		if !ok {
			return
		}
		blob, err := newSarGenerationUsecase(c, uc).DownloadExport(c.Request.Context(), id, exportId)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		defer blob.ReadCloser.Close()
		c.Header("Content-Disposition", `attachment; filename="`+id.String()+"-"+exportId.String()+`.xml"`)
		c.Header("Content-Type", "application/xml")
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, blob.ReadCloser); err != nil {
			_ = c.Error(err)
		}
	}
}

func handleUpdateSarAnalysis(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarUpdateAnalysisInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).UpdateAnalysis(c.Request.Context(), id, in.Version, in.Analysis, in.ExtraInfo)
		sarState(c, http.StatusOK, st, err)
	}
}

func handleListSarCandidates(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		cs, err := newSarGenerationUsecase(c, uc).ListCandidates(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(cs, adaptSarCandidateDto))
	}
}

func handleNavigateSar(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		table, objectId, link := c.Query("table"), c.Query("object_id"), c.Query("link")
		if table == "" || objectId == "" || link == "" {
			presentError(c.Request.Context(), c,
				errors.Wrap(models.BadParameterError, "table, object_id et link sont requis"))
			return
		}
		limit := sarNavigateDefaultLimit
		if raw := c.Query("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				presentError(c.Request.Context(), c, errors.Wrap(models.BadParameterError, "limit invalide"))
				return
			}
			limit = min(n, sarNavigateMaxLimit)
		}
		var cursor *string
		if raw := c.Query("cursor"); raw != "" {
			cursor = &raw
		}
		objs, next, err := newSarGenerationUsecase(c, uc).Navigate(c.Request.Context(), id,
			core.ObjectRef{Table: table, ObjectID: objectId}, link, cursor, limit)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, adaptSarNavigateResponse(objs, next))
	}
}

func handleGetSarSelection(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		objs, err := newSarGenerationUsecase(c, uc).GetSelection(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(objs, dto.AdaptSarSelectedObjectDto))
	}
}

func handleUpdateSarSelection(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		var in dto.SarUpdateSelectionInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).UpdateSelection(c.Request.Context(), id, in.Version,
			adaptSarSelectionAdds(in.Add), dto.AdaptSarSelectionRemovals(in.Remove))
		sarState(c, http.StatusOK, st, err)
	}
}

func sarJob(c *gin.Context, j models.SarGenerationJob, err error) {
	if presentError(c.Request.Context(), c, err) {
		return
	}
	c.JSON(http.StatusAccepted, dto.AdaptSarJobDto(j))
}

func handleGenerateSar(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		j, err := newSarGenerationUsecase(c, uc).StartGeneration(c.Request.Context(), id)
		sarJob(c, j, err)
	}
}

func handleGetSarJob(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "job_id")
		if !ok {
			return
		}
		s, err := newSarGenerationUsecase(c, uc).GetJob(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, adaptSarJobStateDto(s))
	}
}

func handleResumeSarJob(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "job_id")
		if !ok {
			return
		}
		j, err := newSarGenerationUsecase(c, uc).ResumeJob(c.Request.Context(), id)
		sarJob(c, j, err)
	}
}

func handleStartSarReview(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		j, err := newSarGenerationUsecase(c, uc).StartReview(c.Request.Context(), id)
		sarJob(c, j, err)
	}
}

func handleListSarQuestions(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "id")
		if !ok {
			return
		}
		qs, err := newSarGenerationUsecase(c, uc).ListQuestions(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(qs, adaptSarQuestionDto))
	}
}

func handleChooseSarQuestionOption(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "question_id")
		if !ok {
			return
		}
		var in dto.SarChooseOptionInput
		if !sarBindJSON(c, &in) {
			return
		}
		st, err := newSarGenerationUsecase(c, uc).ChooseQuestionOption(c.Request.Context(), id, in.Version, *in.OptionIndex)
		sarState(c, http.StatusOK, st, err)
	}
}

func handleDismissSarQuestion(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := sarUuidParam(c, "question_id")
		if !ok {
			return
		}
		err := newSarGenerationUsecase(c, uc).DismissQuestion(c.Request.Context(), id)
		if presentError(c.Request.Context(), c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
