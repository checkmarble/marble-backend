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

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		organizations, err := usecase.GetOrganizations(ctx)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"organizations": pure_utils.Map(organizations, dto.AdaptOrganizationDto),
		})
	}
}

func handlePostOrganization(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		var data dto.CreateOrganizationBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		organization, err := usecase.CreateOrganization(c.Request.Context(), data.Name)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handleGetOrganizationUsers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		users, err := usecase.GetUsersOfOrganization(c.Request.Context(), organizationID)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"users": pure_utils.Map(users, dto.AdaptUserDto),
		})
	}
}

func handleGetOrganization(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(c.Request.Context(), organizationID)

		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handlePatchOrganization(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationID := c.Param("organization_id")
		var data dto.UpdateOrganizationBodyDto
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		organization, err := usecase.UpdateOrganization(c.Request.Context(), models.UpdateOrganizationInput{
			Id:   organizationID,
			Name: data.Name,
		})

		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"organization": dto.AdaptOrganizationDto(organization),
		})
	}
}

func handleDeleteOrganization(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationID := c.Param("organization_id")

		usecase := usecasesWithCreds(c.Request, uc).NewOrganizationUseCase()
		err := usecase.DeleteOrganization(c.Request.Context(), organizationID)
		if presentError(c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
