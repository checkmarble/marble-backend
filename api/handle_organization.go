package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func (api *API) handleGetOrganizations(c *gin.Context) {
	ctx := c.Request.Context()

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	organizations, err := usecase.GetOrganizations(ctx)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organizations": pure_utils.Map(organizations, dto.AdaptOrganizationDto),
	})
}

func (api *API) handlePostOrganization(c *gin.Context) {
	var data dto.CreateOrganizationBodyDto
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	organization, err := usecase.CreateOrganization(c.Request.Context(), data.Name)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"organization": dto.AdaptOrganizationDto(organization),
	})
}

func (api *API) handleGetOrganizationUsers(c *gin.Context) {
	organizationID := c.Param("organization_id")

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	users, err := usecase.GetUsersOfOrganization(c.Request.Context(), organizationID)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"users": pure_utils.Map(users, dto.AdaptUserDto),
	})
}

func (api *API) handleGetOrganization(c *gin.Context) {
	organizationID := c.Param("organization_id")

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	organization, err := usecase.GetOrganization(c.Request.Context(), organizationID)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"organization": dto.AdaptOrganizationDto(organization),
	})
}

func (api *API) handlePatchOrganization(c *gin.Context) {
	organizationID := c.Param("organization_id")
	var data dto.UpdateOrganizationBodyDto
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	organization, err := usecase.UpdateOrganization(c.Request.Context(), models.UpdateOrganizationInput{
		Id:                         organizationID,
		Name:                       data.Name,
		DatabaseName:               data.DatabaseName,
		ExportScheduledExecutionS3: data.ExportScheduledExecutionS3,
	})

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"organization": dto.AdaptOrganizationDto(organization),
	})
}

func (api *API) handleDeleteOrganization(c *gin.Context) {
	organizationID := c.Param("organization_id")

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	err := usecase.DeleteOrganization(c.Request.Context(), organizationID)
	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
