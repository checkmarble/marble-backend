package api

import (
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type tenantOIDCConfigurationBody struct {
	Issuer       string            `json:"issuer" binding:"required"`
	ClientId     string            `json:"client_id" binding:"required"`
	ClientSecret *string           `json:"client_secret"`
	Scopes       []string          `json:"scopes"`
	ExtraParams  map[string]string `json:"extra_params"`
	EmailClaim   string            `json:"email_claim"`
	SsoEnabled   *bool             `json:"sso_enabled"`
}

type tenantDomainBody struct {
	Domain string `json:"domain" binding:"required"`
}

type tenantDomainResponse struct {
	Id         uuid.UUID `json:"id"`
	TenantId   uuid.UUID `json:"tenant_id"`
	Domain     string    `json:"domain"`
	VerifiedAt time.Time `json:"verified_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func tenantIDFromParam(c *gin.Context) (uuid.UUID, error) {
	tenantId, err := uuid.Parse(c.Param("tenant_id"))
	if err != nil {
		return uuid.Nil, errors.Wrap(models.BadParameterError, "invalid tenant ID")
	}
	return tenantId, nil
}

func adaptTenantDomain(domain models.TenantDomain) tenantDomainResponse {
	return tenantDomainResponse{
		Id: domain.Id, TenantId: domain.TenantId, Domain: domain.Domain, VerifiedAt: domain.VerifiedAt, CreatedAt: domain.CreatedAt,
	}
}

func handleGetTenantOIDCConfiguration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		configuration, err := usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().GetConfiguration(ctx, tenantId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptTenantOIDCConfiguration(configuration))
	}
}

func handlePutTenantOIDCConfiguration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		var body tenantOIDCConfigurationBody
		if err := c.ShouldBindJSON(&body); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		ssoEnabled := true
		if body.SsoEnabled != nil {
			ssoEnabled = *body.SsoEnabled
		}
		err = usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().SaveConfiguration(ctx, tenantId, usecases.TenantOIDCConfigurationInput{
			Issuer: body.Issuer, ClientId: body.ClientId, ClientSecret: body.ClientSecret, Scopes: body.Scopes,
			ExtraParams: body.ExtraParams, EmailClaim: body.EmailClaim, SsoEnabled: ssoEnabled,
		})
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleDeleteTenantOIDCConfiguration(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		if presentError(ctx, c, usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().DeleteConfiguration(ctx, tenantId)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleListTenantDomains(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		domains, err := usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().ListDomains(ctx, tenantId)
		if presentError(ctx, c, err) {
			return
		}
		out := make([]tenantDomainResponse, 0, len(domains))
		for _, domain := range domains {
			out = append(out, adaptTenantDomain(domain))
		}
		c.JSON(http.StatusOK, out)
	}
}

func handlePostTenantDomain(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		var body tenantDomainBody
		if err := c.ShouldBindJSON(&body); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		domain, err := usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().AddDomain(ctx, tenantId, body.Domain)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, adaptTenantDomain(domain))
	}
}

func handleDeleteTenantDomain(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantId, err := tenantIDFromParam(c)
		if presentError(ctx, c, err) {
			return
		}
		domainId, err := uuid.Parse(c.Param("domain_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid domain ID"))
			return
		}
		if presentError(ctx, c, usecasesWithCreds(ctx, uc).NewTenantOIDCUsecase().DeleteDomain(ctx, tenantId, domainId)) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
