package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
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

func handleListEntityAnnotationsForObjects(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		var params dto.EntityAnnotationForObjectsParams

		if err := c.ShouldBindBodyWithJSON(&params); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.EntityAnnotationRequestForObjects{
			OrgId:      creds.OrganizationId,
			ObjectType: params.ObjectType,
			ObjectIds:  params.ObjectIds,
		}

		annotations, err := annotationsUsecase.ListForObjects(ctx, req)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		adapt := func(anns []models.EntityAnnotation) ([]dto.EntityAnnotationDto, error) {
			return pure_utils.MapErr(anns, func(ann models.EntityAnnotation) (dto.EntityAnnotationDto, error) {
				return dto.AdaptEntityAnnotation(ann)
			})
		}

		out, err := pure_utils.MapValuesErr(annotations, adapt)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleGetAnnotationByCase(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		caseId := c.Param("case_id")

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.CaseEntityAnnotationRequest{
			OrgId:  creds.OrganizationId,
			CaseId: caseId,
		}

		if t := c.Query("type"); t != "" {
			annotationType := models.EntityAnnotationFrom(t)

			if annotationType == models.EntityAnnotationUnknown {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid annotation type"))
				return
			}

			req.AnnotationType = &annotationType
		}

		annotations, err := annotationsUsecase.ListForCase(ctx, req)
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
			CaseId:         payload.CaseId,
			AnnotationType: models.EntityAnnotationFrom(payload.Type),
			Payload:        parsedPayload,
			AnnotatedBy:    &creds.ActorIdentity.UserId,
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
			AnnotatedBy: &creds.ActorIdentity.UserId,
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

func handleDeleteEntityAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)
		annotationId := c.Param("annotationId")

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		req := models.AnnotationByIdRequest{
			OrgId:        creds.OrganizationId,
			AnnotationId: annotationId,
		}

		if err := annotationsUsecase.DeleteAnnotation(ctx, req); err != nil {
			presentError(ctx, c, err)
			return
		}

		c.Status(http.StatusNoContent)
	}
}
