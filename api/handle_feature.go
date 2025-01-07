package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleListFeatures(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewFeatureUseCase()
		features, err := usecase.ListAllFeatures(ctx)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"features": pure_utils.Map(features, dto.AdaptFeatureDto)})
	}
}

func handleCreateFeature(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.CreateFeatureBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewFeatureUseCase()
		feature, err := usecase.CreateFeature(ctx, models.CreateFeatureAttributes{
			Name: data.Name,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"feature": dto.AdaptFeatureDto(feature)})
	}
}

type FeatureUriInput struct {
	FeatureId string `uri:"feature_id" binding:"required,uuid"`
}

func handleGetFeature(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var featureInput FeatureUriInput
		if err := c.ShouldBindUri(&featureInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewFeatureUseCase()
		feature, err := usecase.GetFeatureById(ctx, featureInput.FeatureId)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"feature": dto.AdaptFeatureDto(feature)})
	}
}

func handleUpdateFeature(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var featureInput FeatureUriInput
		if err := c.ShouldBindUri(&featureInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		var data dto.UpdateFeatureBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewFeatureUseCase()
		feature, err := usecase.UpdateFeature(ctx, models.UpdateFeatureAttributes{
			Name: data.Name,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"feature": dto.AdaptFeatureDto(feature)})
	}
}

func handleDeleteFeature(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var featureInput FeatureUriInput
		if err := c.ShouldBindUri(&featureInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewFeatureUseCase()
		err = usecase.DeleteFeature(ctx, organizationId, featureInput.FeatureId)

		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
