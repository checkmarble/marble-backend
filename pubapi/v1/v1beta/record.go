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

func HandleGetRecordAnnotations(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		recordType := c.Param("recordType")
		recordId := c.Param("recordId")

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()
		annotations, err := uc.List(
			ctx,
			models.EntityAnnotationRequest{
				OrgId:      orgId,
				ObjectType: recordType,
				ObjectId:   recordId,
			},
		)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		response, err := pure_utils.MapErr(annotations, dto.AdaptRecordAnnotationDto)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		types.NewResponse(response).Serve(c, http.StatusOK)
	}
}

func HandleAttachRecordAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		recordType := c.Param("recordType")
		recordId := c.Param("recordId")

		var payload params.AttachRecordAnnotationParams
		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}
		createRequests := make([]models.CreateEntityAnnotationRequest, len(payload.Annotations))
		for i, item := range payload.Annotations {
			annotationType := models.EntityAnnotationFrom(item.Type)
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

			parsedPayload, err := params.DecodeAnnotationPayload(annotationType, item.Payload)
			if err != nil {
				types.NewErrorResponse().WithError(err).Serve(c)
				return
			}
			createRequests[i] = models.CreateEntityAnnotationRequest{
				OrgId:          orgId,
				ObjectType:     recordType,
				ObjectId:       recordId,
				AnnotationType: annotationType,
				Payload:        parsedPayload,
			}
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		annotation, err := uc.AttachByBatch(ctx, createRequests)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		response, err := pure_utils.MapErr(annotation, dto.AdaptRecordAnnotationDto)
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

		recordType := c.Param("recordType")
		recordId := c.Param("recordId")

		var payload params.AttachRecordFileAnnotationParams

		if err := c.ShouldBind(&payload); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		annotations, err := uc.AttachFile(
			ctx,
			models.CreateEntityAnnotationRequest{
				OrgId:          orgId,
				ObjectType:     recordType,
				ObjectId:       recordId,
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

		response, err := pure_utils.MapErr(annotations, dto.AdaptRecordAnnotationDto)
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

		types.Redirect(c, downloadUrl)
	}
}

func HandleDeleteEntityAnnotations(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		var params params.RecordDeleteAnnotationsParams
		if err := c.ShouldBindJSON(&params); err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		requests := make([]models.AnnotationByIdRequest, len(params.Ids))
		for i, id := range params.Ids {
			requests[i] = models.AnnotationByIdRequest{
				OrgId:        orgId,
				AnnotationId: id,
			}
		}

		uc := pubapi.UsecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()

		err = uc.DeleteAnnotationByBatch(ctx, requests)
		if err != nil {
			types.NewErrorResponse().WithError(err).Serve(c)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
