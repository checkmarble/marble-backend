package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// aiPromptsLicenseKey extracts the caller's license key from the "Authorization: Bearer <key>"
// header.
func aiPromptsLicenseKey(c *gin.Context) (string, error) {
	key, err := utils.ParseAuthorizationBearerHeader(c.Request.Header)
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", models.UnAuthorizedError
	}
	return key, nil
}

// handleAiPromptsDownload streams a zip of the whole AI prompt bundle the caller's license is
// entitled to, for the exact prompts_version requested (the caller always asks for one precise
// version — its own hardcoded default, or an operator-configured pin — never "give me the
// latest"; see PromptServingUsecase for the resolution rules). The zip contains only the
// prompt files themselves — no manifest or version is reported back, since the caller already
// knows which version it asked for. Only reachable on Marble SaaS.
func handleAiPromptsDownload(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		licenseKey, err := aiPromptsLicenseKey(c)
		if presentError(ctx, c, err) {
			return
		}

		version := c.Query("prompts_version")

		usecase := uc.NewPromptServingUsecase()
		zipReader, err := usecase.DownloadPrompts(ctx, licenseKey, version)
		if presentError(ctx, c, err) {
			return
		}

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", `attachment; filename="ai-prompts.zip"`)
		if _, err := io.Copy(c.Writer, zipReader); err != nil {
			presentError(ctx, c, err)
			return
		}
		c.Status(http.StatusOK)
	}
}
