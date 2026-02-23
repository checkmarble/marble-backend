package v1beta

import (
	"net/http"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/dto"
	"github.com/checkmarble/marble-backend/pubapi/v1/params"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func HandleGetClientDataAnnotations(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		objectType := c.Param("objectType")
		objectId := c.Param("objectId")

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()
		annotations, err := uc.List(
			ctx,
			models.EntityAnnotationRequest{
				OrgId:      orgId,
				ObjectType: objectType,
				ObjectId:   objectId,
			},
		)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(pure_utils.Map(annotations, dto.AdaptClientDataAnnotationDto)).Serve(c, http.StatusOK)
	}
}

func HandleAttachClientDataAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		objectType := c.Param("objectType")
		objectId := c.Param("objectId")

		var payload params.AttachClientDataAnnotationParams

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		annotationType := models.EntityAnnotationFrom(payload.Type)
		if annotationType == models.EntityAnnotationUnknown {
			types.NewErrorResponse().WithError(errors.WithDetail(
				models.BadParameterError, "invalid annotation type")).Serve(c)
			return
		}
		if annotationType == models.EntityAnnotationFile {
			types.NewErrorResponse().WithError(
				errors.WithDetail(models.BadParameterError,
					"cannot use generic annotation endpoint to add file annotation"),
			).Serve(c)
			return
		}

		parsedPayload, err := gdto.DecodeEntityAnnotationPayload(annotationType, payload.Payload)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		req := models.CreateEntityAnnotationRequest{
			OrgId:          orgId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			AnnotationType: annotationType,
			Payload:        parsedPayload,
		}

		annotation, err := uc.Attach(ctx, req)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.AdaptClientDataAnnotationDto(annotation)).Serve(c, http.StatusCreated)
	}
}

func HandleDeleteClientDataAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		annotationId, err := types.UuidParam(c, "id")
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		err = uc.DeleteAnnotation(
			ctx,
			models.AnnotationByIdRequest{
				OrgId:        orgId,
				AnnotationId: annotationId.String(),
			},
		)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
