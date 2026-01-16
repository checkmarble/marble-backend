package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
		orgId, err := uuid.Parse(organizationID)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(ctx, orgId)

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
		organizationIdStr := c.Param("organization_id")
		var data dto.UpdateOrganizationBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		orgId, err := uuid.Parse(organizationIdStr)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		organization, err := usecase.UpdateOrganization(ctx, models.UpdateOrganizationInput{
			Id:                      orgId,
			DefaultScenarioTimezone: data.DefaultScenarioTimezone,
			ScreeningConfig: models.OrganizationOpenSanctionsConfigUpdateInput{
				MatchThreshold: data.SanctionsThreshold,
				MatchLimit:     data.SanctionsLimit,
			},
			AutoAssignQueueLimit:    data.AutoAssignQueueLimit,
			SentryReplayEnabled:     data.SentryReplayEnabled,
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
		organizationIdStr := c.Param("organization_id")
		orgId, err := uuid.Parse(organizationIdStr)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		err = usecase.DeleteOrganization(ctx, orgId)
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

		organizationIdFromPathStr := c.Param("organization_id")
		organizationIdFromQueryStr := c.Query("organization_id")
		var organizationIdFromPath uuid.UUID
		var organizationIdFromQuery uuid.UUID
		var err error
		if organizationIdFromPathStr != "" {
			organizationIdFromPath, err = uuid.Parse(organizationIdFromPathStr)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
		}
		if organizationIdFromQueryStr != "" {
			organizationIdFromQuery, err = uuid.Parse(organizationIdFromQueryStr)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
		}

		orgId := creds.OrganizationId
		if orgId == uuid.Nil && organizationIdFromQuery != uuid.Nil {
			orgId = organizationIdFromQuery
		} else if orgId == uuid.Nil && organizationIdFromPath != uuid.Nil {
			orgId = organizationIdFromPath
		}
		if orgId == uuid.Nil {
			c.Status(http.StatusBadRequest)
			return
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
		organizationIdStr := c.Param("organization_id")
		var organizationId uuid.UUID
		if organizationIdStr != "" {
			var err error
			organizationId, err = uuid.Parse(organizationIdStr)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
		}
		var data dto.UpdateOrganizationFeatureAccessBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		err := usecase.UpdateOrganizationFeatureAccess(ctx,
			dto.AdaptUpdateOrganizationFeatureAccessInput(data, organizationId))
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleUpdateOrganizationSubnets(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var subnetUpdate dto.OrganizationSubnetsDto

		if err := c.ShouldBindBodyWithJSON(&subnetUpdate); presentError(ctx, c, err) {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		subnets, err := uc.UpdateOrganizationSubnets(ctx,
			pure_utils.Map(subnetUpdate.Subnets, func(s dto.SubnetDto) net.IPNet { return s.IPNet }))

		if err != nil && (errors.Is(err, usecases.ErrClientOutsideOfAllowedNetworks) ||
			errors.Is(err, usecases.ErrRealClientIpNotPresent)) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": err.Error(),
			})
			return
		}

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(subnets, dto.AdaptOrganizationSubnet))
	}
}
