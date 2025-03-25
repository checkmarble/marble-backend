package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func handleListEntityAnnotations(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		objectType := c.Param("object_type")
		objectId := c.Param("object_id")

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.EntityAnnotationRequest{
			OrgId:      creds.OrganizationId,
			ObjectType: objectType,
			ObjectId:   objectId,
		}

		if t := c.Query("type"); t != "" {
			annotationType := models.EntityAnnotationFrom(t)

			if annotationType == models.EntityAnnotationUnknown {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid annotation type"))
				return
			}

			req.AnnotationType = &annotationType
		}

		annotations, err := annotationsUsecase.List(ctx, req)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		out, err := pure_utils.MapErr(annotations, dto.AdaptEntityAnnotation)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleCreateEntityAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		objectType := c.Param("object_type")
		objectId := c.Param("object_id")

		var payload dto.PostEntityAnnotationDto

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		parsedPayload, err := dto.DecodeEntityAnnotationPayload(
			models.EntityAnnotationFrom(payload.Type), payload.Payload)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		req := models.CreateEntityAnnotationRequest{
			OrgId:          creds.OrganizationId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			AnnotationType: models.EntityAnnotationFrom(payload.Type),
			Payload:        parsedPayload,
		}

		if req.AnnotationType == models.EntityAnnotationUnknown {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid annotation type"))
			return
		}
		if req.AnnotationType == models.EntityAnnotationFile {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"cannot use generic annotation endpoint to add file annotation"))
			return
		}
		if err := binding.Validator.ValidateStruct(req.Payload); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				fmt.Sprintf("invalid payload for annotation type: %v", err)))
			return
		}

		annotation, err := annotationsUsecase.Attach(ctx, req)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		out, err := dto.AdaptEntityAnnotation(annotation)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleCreateEntityFileAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		objectType := c.Param("object_type")
		objectId := c.Param("object_id")

		var payload dto.PostEntityFileAnnotationDto

		if err := c.ShouldBind(&payload); err != nil {
			presentError(ctx, c, err)
			return
		}
		if len(payload.Files) == 0 {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"at least one file should be provided"))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.CreateEntityAnnotationRequest{
			OrgId:          creds.OrganizationId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			AnnotationType: models.EntityAnnotationFile,
			Payload: models.EntityAnnotationFilePayload{
				Caption: payload.Caption,
			},
		}

		if err := binding.Validator.ValidateStruct(req.Payload); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				fmt.Sprintf("invalid payload for annotation type: %v", err)))
			return
		}

		annotation, err := annotationsUsecase.AttachFile(ctx, req, payload.Files)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		out, err := dto.AdaptEntityAnnotation(annotation)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleGetEntityFileAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		annotationId := c.Param("annotationId")
		partId := c.Param("partId")

		var payload dto.PostEntityFileAnnotationDto

		if err := c.ShouldBind(&payload); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.AnnotationByIdRequest{
			OrgId:        creds.OrganizationId,
			AnnotationId: annotationId,
		}

		annotation, err := annotationsUsecase.GetFileDownloadUrl(ctx, req, partId)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusCreated, gin.H{"url": annotation})
	}
}
