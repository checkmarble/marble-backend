package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"

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
	"github.com/checkmarble/marble-backend/utils"
)

// HTTP handlers of the SAR declarations. Rulings: the request DTO maps only path+value to core.Fill; every error goes
// through presentSarError, which writes a stable error_code and a clean French message (and details.issues when the
// refusal carries issues): Conflict 409, BadParameter 400, Forbidden 403, NotFound 404, invalid UUID 400; the usecase
// comes from newSarGenerationUsecase(c, uc).

const (
	sarNavigateDefaultLimit = 100
	sarNavigateMaxLimit     = 100
)

// sarUuidParam parses a UUID path parameter; an invalid one is a 400.
func sarUuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		presentSarError(c,
			models.NewSarErrorf(models.BadParameterError, models.SarCodeInvalidRequest, "identifiant invalide pour %s", name))
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
	msg := "corps de requête invalide"
	if errors.As(err, &verr) {
		msg += " : champ invalide ou manquant (" + strings.Join(pure_utils.Map(verr, func(fe validator.FieldError) string {
			return fe.Field()
		}), ", ") + ")"
	}
	presentSarError(c, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, msg))
	return false
}

// sarIssuesCarrier is a refused write exposing its structured issues.
type sarIssuesCarrier interface {
	error
	SarIssues() []core.Issue
	SarCode() models.SarErrorCode
}

// presentSarError is presentError for the SAR generation endpoints. A refusal
// carrying issues is a 400 with details.issues (same shape for every endpoint);
// any other SarError is written with its own status (from the sentinel it
// wraps), its stable error_code and its clean French message, never with
// wrapper text of the layers it crossed (transaction, sentinel). Other errors
// fall back to presentError, so unexpected failures stay a 500.
func presentSarError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	ctx := c.Request.Context()
	var carrier sarIssuesCarrier
	if errors.Is(err, models.BadParameterError) && errors.As(err, &carrier) {
		utils.LoggerFromContext(ctx).InfoContext(ctx, "SAR generation refusal", "code", carrier.SarCode(), "error", carrier.Error())
		c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
			Message:   carrier.Error(),
			ErrorCode: dto.ErrorCode(carrier.SarCode()),
			Details:   dto.SarIssuesDetails{Issues: pure_utils.Map(carrier.SarIssues(), dto.AdaptSarIssueDto)},
		})
		return true
	}
	var sarErr *models.SarError
	if errors.As(err, &sarErr) {
		if status, ok := sarStatusOf(sarErr); ok {
			sarRefusal(c, status, sarErr.Code, sarErr.Message, err)
			return true
		}
		// A kind that maps to no status: never leak the detail.
		utils.LoggerFromContext(ctx).ErrorContext(ctx, "SAR generation error of an unmapped kind", "error", err.Error())
		sarRefusal(c, http.StatusInternalServerError, models.SarCodeInternal,
			"une erreur inattendue est survenue ; réessayez ou contactez le support", err)
		return true
	}
	// Bare sentinels coming from the layers below (repositories, case security):
	// same status as presentError, but a code and a generic French message, never
	// the wrapped Go chain.
	for _, b := range []struct {
		kind    error
		status  int
		code    models.SarErrorCode
		message string
	}{
		{models.BadParameterError, http.StatusBadRequest, models.SarCodeInvalidRequest, "requête invalide"},
		{models.UnprocessableEntityError, http.StatusUnprocessableEntity, models.SarCodeUnprocessable, "requête impossible à traiter"},
		{models.UnAuthorizedError, http.StatusUnauthorized, models.SarCodeUnauthorized, "authentification requise"},
		{models.ForbiddenError, http.StatusForbidden, models.SarCodeForbidden, "accès refusé"},
		{models.MissingLicenseEntitlementError, http.StatusForbidden, models.SarCodeForbidden, "accès refusé"},
		{models.NotFoundError, http.StatusNotFound, models.SarCodeNotFound, "élément introuvable"},
		{models.ConflictError, http.StatusConflict, models.SarCodeConflict, "conflit avec l'état actuel de la déclaration"},
	} {
		if errors.Is(err, b.kind) {
			sarRefusal(c, b.status, b.code, b.message, err)
			return true
		}
	}
	return presentError(ctx, c, err)
}

func sarStatusOf(e *models.SarError) (int, bool) {
	switch {
	case errors.Is(e, models.BadParameterError):
		return http.StatusBadRequest, true
	case errors.Is(e, models.ForbiddenError):
		return http.StatusForbidden, true
	case errors.Is(e, models.NotFoundError):
		return http.StatusNotFound, true
	case errors.Is(e, models.ConflictError):
		return http.StatusConflict, true
	}
	return 0, false
}

func sarRefusal(c *gin.Context, status int, code models.SarErrorCode, message string, cause error) {
	ctx := c.Request.Context()
	utils.LoggerFromContext(ctx).InfoContext(ctx, "SAR generation refusal", "code", code, "error", cause.Error())
	c.JSON(status, dto.APIErrorResponse{Message: message, ErrorCode: dto.ErrorCode(code)})
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
			presentSarError(c, models.NewSarError(models.NotFoundError, models.SarCodeUnknownFormat, "format inconnu"))
			return
		}
		c.JSON(http.StatusOK, dto.AdaptSarSpecDto(f))
	}
}

func handleGetSarOrgSettings(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, issues, err := newSarGenerationUsecase(c, uc).GetOrgSettings(c.Request.Context(), c.Param("family"))
		if presentSarError(c, err) {
			return
		}
		ready, err := newSarGenerationUsecase(c, uc).SettingsReadiness(c.Request.Context(), c.Param("family"), s.Values)
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
			return
		}
		ready, err := newSarGenerationUsecase(c, uc).SettingsReadiness(c.Request.Context(), c.Param("family"), s.Values)
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
			presentSarError(c,
				models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, "table, object_id et link sont requis"))
			return
		}
		limit := sarNavigateDefaultLimit
		if raw := c.Query("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 {
				presentSarError(c, models.NewSarError(models.BadParameterError, models.SarCodeInvalidRequest, "limit invalide"))
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
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
	if presentSarError(c, err) {
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
		if presentSarError(c, err) {
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
		if presentSarError(c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(qs, adaptSarQuestionDto))
	}
}

// handleChooseSarQuestionOption applies an option of a question. Version
// contract: choosing an option that writes (including one that adds a list item)
// bumps the declaration version. Choosing the option that declines an addition,
// or an addition whose value the list already holds, closes the question as
// resolved without writing: the response is 200 and keeps the same version, so
// the client must not expect it to grow. The client still sends the version it
// saw: a stale one is a 409, like for any option. Unlike dismiss, which takes
// no version.
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
		if presentSarError(c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
