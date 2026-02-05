package v1beta

import (
	"errors"
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func HandleIngestObject(uc usecases.Usecases, batch bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		objectType := c.Param("objectType")

		var p params.IngestionParams

		if err := c.ShouldBindQuery(&p); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		object, err := io.ReadAll(c.Request.Body)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		usecase := pubapi.UsecasesWithCreds(ctx, uc).NewIngestionUseCase()

		f := usecase.IngestObject
		if batch {
			f = usecase.IngestObjects
		}

		ingestionOptions := models.IngestionOptions{
			ShouldMonitor: p.MonitorObjects,
			ShouldScreen:  !p.SkipInitialScreening,
		}

		if p.MonitorObjects {
			ingestionOptions.ContinuousScreeningId = uuid.MustParse(p.ContinuousConfigId)
		}

		partial := c.Request.Method == http.MethodPatch
		ingestedCount, err := f(ctx, orgId, objectType, object, ingestionOptions,
			payload_parser.WithAllowedPatch(partial), payload_parser.DisallowUnknownFields())
		if err != nil {
			var validationError models.IngestionValidationErrors

			if errors.As(err, &validationError) {
				types.
					NewErrorResponse().
					WithError(err).
					WithErrorCode(string(dto.SchemaMismatchError)).
					WithErrorMessage("input validation error").
					WithErrorDetails(validationError).
					Serve(c)
				return
			}

			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		if ingestedCount == 0 {
			c.Status(http.StatusOK)
			return
		}

		c.Status(http.StatusCreated)
	}
}
