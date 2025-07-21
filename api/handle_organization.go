package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleGetOrganizations(uc usecases.Usecases) func(c *gin.Context) {
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

func handlePostOrganization(uc usecases.Usecases) func(c *gin.Context) {
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

func handleGetOrganization(uc usecases.Usecases) func(c *gin.Context) {
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

func handlePatchOrganization(uc usecases.Usecases) func(c *gin.Context) {
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
			ScreeningConfig: models.OrganizationOpenSanctionsConfigUpdateInput{
				MatchThreshold: data.SanctionsThreshold,
				MatchLimit:     data.SanctionsLimit,
			},
			AutoAssignQueueLimit: data.AutoAssignQueueLimit,
		})

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handleDeleteOrganization(uc usecases.Usecases) func(c *gin.Context) {
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

func handleGetOrganizationFeatureAccess(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		organizationIdFromPath := c.Param("organization_id")
		organizationIdFromQuery := c.Query("organization_id")
		orgId := creds.OrganizationId
		if orgId == "" && organizationIdFromQuery != "" {
			orgId = organizationIdFromQuery
		} else if orgId == "" && organizationIdFromPath != "" {
			orgId = organizationIdFromPath
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		featureAccess, err := usecase.GetOrganizationFeatureAccess(ctx, orgId, utils.Ptr(creds.ActorIdentity.UserId))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"feature_access": dto.AdaptOrganizationFeatureAccessDto(featureAccess),
		})
	}
}

func handlePatchOrganizationFeatureAccess(uc usecases.Usecases) func(c *gin.Context) {
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
