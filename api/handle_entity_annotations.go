package api

import (
	"encoding/json"
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
			OrgId:          creds.OrganizationId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			LoadThumbnails: c.Query("load_thumbnails") == "true",
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

		out, err := dto.AdaptGroupedEntityAnnotations(
			models.GroupAnnotationsByType(annotations))
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		c.JSON(http.StatusOK, out)
	}
}

func handleGetEntityAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		id := c.Param("id")

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		annotations, err := annotationsUsecase.Get(ctx, creds.OrganizationId, id)
		if err != nil {
			presentError(ctx, c, err)
			return
		}

		out, err := dto.AdaptEntityAnnotation(annotations)
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

		annotationType := models.EntityAnnotationFrom(payload.Type)
		if annotationType == models.EntityAnnotationUnknown {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid annotation type"))
			return
		}
		if annotationType == models.EntityAnnotationFile {
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"cannot use generic annotation endpoint to add file annotation"))
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		annotationsUsecase := uc.NewEntityAnnotationUsecase()

		var annotation models.EntityAnnotation

		switch annotationType {
		case models.EntityAnnotationRiskTopic:
			// Risk topic annotations have special upsert semantics (one per object, merge topics)
			var riskTopicInput dto.RiskTopicAnnotationInputDto
			if err := json.Unmarshal(payload.Payload, &riskTopicInput); err != nil {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
				return
			}

			upsertInput, err := riskTopicInput.Adapt(creds.OrganizationId, objectType, objectId)
			if err != nil {
				presentError(ctx, c, err)
				return
			}

			annotation, err = annotationsUsecase.UpsertRiskTopicAnnotation(ctx, upsertInput)
			if err != nil {
				presentError(ctx, c, err)
				return
			}
		default:
			parsedPayload, err := dto.DecodeEntityAnnotationPayload(annotationType, payload.Payload)
			if err != nil {
				presentError(ctx, c, err)
				return
			}

			req := models.CreateEntityAnnotationRequest{
				OrgId:          creds.OrganizationId,
				ObjectType:     objectType,
				ObjectId:       objectId,
				CaseId:         payload.CaseId,
				AnnotationType: annotationType,
				Payload:        parsedPayload,
				AnnotatedBy:    &creds.ActorIdentity.UserId,
			}

			annotation, err = annotationsUsecase.Attach(ctx, req)
			if err != nil {
				presentError(ctx, c, err)
				return
			}
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
			CaseId:         payload.CaseId,
			ObjectType:     objectType,
			ObjectId:       objectId,
			AnnotationType: models.EntityAnnotationFile,
			Payload: models.EntityAnnotationFilePayload{
				Caption: payload.Caption,
			},
			AnnotatedBy: &creds.ActorIdentity.UserId,
		}

		annotations, err := annotationsUsecase.AttachFile(ctx, req, payload.Files)
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

func handleGetEntityFileAnnotation(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)

		annotationId := c.Param("annotationId")
		partId := c.Param("partId")

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
