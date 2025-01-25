package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
)

func handleGetOrganizations(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		organizations, err := usecase.GetOrganizations(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"organizations": pure_utils.Map(organizations, dto.AdaptOrganizationDto),
		})
	}
}

func handlePostOrganization(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.CreateOrganizationBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		organization, err := usecase.CreateOrganization(ctx, data.Name)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handleGetOrganization(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(ctx, organizationID)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handlePatchOrganization(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")
		var data dto.UpdateOrganizationBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		organization, err := usecase.UpdateOrganization(ctx, models.UpdateOrganizationInput{
			Id:                      organizationID,
			DefaultScenarioTimezone: data.DefaultScenarioTimezone,
			SanctionCheckConfig: models.OrganizationOpenSanctionsConfig{
				Datasets:       data.SanctionCheckDatasets,
				MatchThreshold: data.SanctionCheckThreshold,
				MatchLimit:     data.SanctionCheckLimit,
			},
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handleDeleteOrganization(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		err := usecase.DeleteOrganization(ctx, organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleGetOrganizationFeatureAccess(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		featureAccess, err := usecase.GetOrganizationFeatureAccess(ctx, organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"feature_access": dto.AdaptOrganizationFeatureAccessDto(featureAccess),
		})
	}
}

func handlePatchOrganizationFeatureAccess(uc usecases.Usecaser) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")
		var data dto.UpdateOrganizationFeatureAccessBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		err := usecase.UpdateOrganizationFeatureAccess(ctx,
			dto.AdaptUpdateOrganizationFeatureAccessInput(data, organizationID))
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
