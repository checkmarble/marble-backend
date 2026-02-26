package v1beta

import (
	"net/http"

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

		response, err := pure_utils.MapErr(annotations, dto.AdaptClientDataAnnotationDto)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(response).Serve(c, http.StatusOK)
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
				types.ErrInvalidPayload, "invalid annotation type")).Serve(c)
			return
		}
		if annotationType == models.EntityAnnotationFile {
			types.NewErrorResponse().WithError(
				errors.WithDetail(types.ErrInvalidPayload,
					"cannot use generic annotation endpoint to add file annotation"),
			).Serve(c)
			return
		}

		parsedPayload, err := params.DecodeAnnotationPayload(annotationType, payload.Payload)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		annotation, err := uc.Attach(ctx, models.CreateEntityAnnotationRequest{
			OrgId:          orgId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			AnnotationType: annotationType,
			Payload:        parsedPayload,
		})
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		response, err := dto.AdaptClientDataAnnotationDto(annotation)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(response).Serve(c, http.StatusCreated)
	}
}

func HandleCreateEntityFileAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		objectType := c.Param("objectType")
		objectId := c.Param("objectId")

		var payload params.AttachClientDataFileAnnotationParams

		if err := c.ShouldBind(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		if len(payload.Files) == 0 {
			types.NewErrorResponse().WithError(
				errors.WithDetail(types.ErrInvalidPayload,
					"at least one file should be provided"),
			).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		annotations, err := uc.AttachFile(
			ctx,
			models.CreateEntityAnnotationRequest{
				OrgId:          orgId,
				ObjectType:     objectType,
				ObjectId:       objectId,
				AnnotationType: models.EntityAnnotationFile,
				Payload: models.EntityAnnotationFilePayload{
					Caption: payload.Caption,
				},
			},
			payload.Files,
		)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		response, err := pure_utils.MapErr(annotations, dto.AdaptClientDataAnnotationDto)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(response).Serve(c, http.StatusCreated)
	}
}

func HandleGetEntityFileAnnotation(uc usecases.Usecases) gin.HandlerFunc {
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

		partId := c.Param("partId")
		if partId == "" {
			types.NewErrorResponse().WithError(errors.WithDetail(
				types.ErrInvalidPayload, "partId is required",
			)).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		downloadUrl, err := uc.GetFileDownloadUrl(
			ctx,
			models.AnnotationByIdRequest{
				OrgId:        orgId,
				AnnotationId: annotationId.String(),
			}, partId,
		)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(dto.ClientDataFileUrl{Url: downloadUrl}).Serve(c)
	}
}
