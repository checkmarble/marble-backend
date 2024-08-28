package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
)

func handleListLicenses(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		usecase := usecasesWithCreds(c.Request, uc).NewLicenseUsecase()
		licenses, err := usecase.ListLicenses(c.Request.Context())
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"licenses": pure_utils.Map(licenses, dto.AdaptLicenseDto),
		})
	}
}

func handleCreateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		var data dto.CreateLicenseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewLicenseUsecase()
		license, err := usecase.CreateLicense(c.Request.Context(), dto.AdaptCreateLicenseInput(data))
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleGetLicenseById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		licenseId := c.Param("license_id")

		usecase := usecasesWithCreds(c.Request, uc).NewLicenseUsecase()
		license, err := usecase.GetLicenseById(c.Request.Context(), licenseId)
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleUpdateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("license_id")

		var data dto.UpdateLicenseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewLicenseUsecase()
		license, err := usecase.UpdateLicense(c.Request.Context(), dto.AdaptUpdateLicenseInput(id, data))
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleValidateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		licenseKey := c.Param("license_key")

		usecase := uc.NewLicenseUsecase()
		licenseValidation, err := usecase.ValidateLicense(c.Request.Context(), strings.TrimPrefix(licenseKey, "/"))
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptLicenseValidationDto(licenseValidation))
	}
}
