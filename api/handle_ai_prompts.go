package api

import (
	"io"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Extract the caller's license
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
// entitled to, resolved for the caller's prompts_version (its own product Major.Minor).
// Resolution is a backward search: exact Major.Minor
// match wins, else the nearest earlier published one; never a newer one. The zip contains only the prompt files themselves.
// Only reachable on Marble SaaS.
func handleAiPromptsDownload(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		licenseKey, err := aiPromptsLicenseKey(c)
		if presentError(ctx, c, err) {
			return
		}

		version := c.Query("prompts_version")
		if version == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "prompts_version in query is required"))
			return
		}

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
