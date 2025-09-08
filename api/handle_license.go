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
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewLicenseUsecase()
		licenses, err := usecase.ListLicenses(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"licenses": pure_utils.Map(licenses, dto.AdaptLicenseDto),
		})
	}
}

func handleCreateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.CreateLicenseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewLicenseUsecase()
		license, err := usecase.CreateLicense(ctx, dto.AdaptCreateLicenseInput(data))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleGetLicenseById(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		licenseId := c.Param("license_id")

		usecase := usecasesWithCreds(ctx, uc).NewLicenseUsecase()
		license, err := usecase.GetLicenseById(ctx, licenseId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleUpdateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("license_id")

		var data dto.UpdateLicenseBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewLicenseUsecase()
		license, err := usecase.UpdateLicense(ctx, dto.AdaptUpdateLicenseInput(id, data))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"license": dto.AdaptLicenseDto(license),
		})
	}
}

func handleValidateLicense(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		licenseKey := c.Param("license_key")
		deploymentId := c.Query("deployment_id")

		// Should we check if the deployment is an UUID if provided and return an error if not?
		// if deploymentId != "" {
		// 	if _, err := uuid.Parse(deploymentId); err != nil {
		// 		presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid deployment_id format"))
		// 		return
		// 	}
		// }

		usecase := uc.NewLicenseUsecase()
		licenseValidation, err := usecase.ValidateLicense(
			ctx,
			strings.TrimPrefix(licenseKey, "/"),
			deploymentId,
		)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptLicenseValidationDto(licenseValidation))
	}
}

func handleIsSSOEnabled(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		usecase := uc.NewLicenseUsecase()
		c.JSON(http.StatusOK, gin.H{
			"is_sso_enabled": usecase.HasSsoEnabled(),
		})
	}
}
