package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
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

func handleGetOrganizationUsers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(ctx, uc).NewOrganizationUseCase()
		users, err := usecase.GetUsersOfOrganization(ctx, organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"users": pure_utils.Map(users, dto.AdaptUserDto),
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
			Id:   organizationID,
			Name: data.Name,
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
