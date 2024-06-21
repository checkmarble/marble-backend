package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func (api *API) handleListLicenses(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewLicenseUsecase()
	licenses, err := usecase.ListLicenses(c.Request.Context())
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"licenses": pure_utils.Map(licenses, dto.AdaptLicenseDto),
	})
}

func (api *API) handleCreateLicense(c *gin.Context) {
	var data dto.CreateLicenseBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewLicenseUsecase()
	license, err := usecase.CreateLicense(c.Request.Context(), dto.AdaptCreateLicenseInput(data))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"license": dto.AdaptLicenseDto(license),
	})
}

func (api *API) handleGetLicenseById(c *gin.Context) {
	licenseId := c.Param("license_id")

	usecase := api.UsecasesWithCreds(c.Request).NewLicenseUsecase()
	license, err := usecase.GetLicenseById(c.Request.Context(), licenseId)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"license": dto.AdaptLicenseDto(license),
	})
}

func (api *API) handleUpdateLicense(c *gin.Context) {
	id := c.Param("license_id")

	var data dto.UpdateLicenseBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewLicenseUsecase()
	license, err := usecase.UpdateLicense(c.Request.Context(), dto.AdaptUpdateLicenseInput(id, data))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"license": dto.AdaptLicenseDto(license),
	})
}

func (api *API) handleValidateLicense(c *gin.Context) {
	licenseKey := c.Param("license_key")

	usecase := api.usecases.NewLicenseUsecase()
	licenseValidation, err := usecase.ValidateLicense(c.Request.Context(), licenseKey)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptLicenseValidationDto(licenseValidation))
}
