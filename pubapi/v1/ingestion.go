package v1

import (
	"io"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
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

		partial := c.Request.Method == http.MethodPatch
		ingestedCount, err := f(ctx, orgId, objectType, object, models.IngestionOptions{
			ShouldScreen: true,
		},
			payload_parser.WithAllowedPatch(partial), payload_parser.DisallowUnknownFields())
		if err != nil {
			switch batch {
			case true:
				if types.PresentMultipleObjectsValidationError(c, err) {
					return
				}
			case false:
				if types.PresentSingleObjectValidationError(c, err) {
					return
				}
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
