package auth

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/google/uuid"
)

type marbleRepository interface {
	GetApiKeyByHash(ctx context.Context, hash []byte) (models.ApiKey, error)
	GetOrganizationByID(ctx context.Context, organizationID uuid.UUID) (models.Organization, error)
	UserByEmail(ctx context.Context, email string) (models.User, error)
	ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.RoleBinding, error)
	UpdateUserProfileFromClaims(
		ctx context.Context,
		user models.User,
		profile models.IdentityUpdatableClaims,
	) (models.User, error)
}

type encoder interface {
	EncodeMarbleToken(issuer string, expirationTime time.Time, creds models.Credentials) (string, error)
}

type TokenGenerator interface {
	GenerateToken(ctx context.Context, creds Credentials, intoCredentials models.IntoCredentials,
		claims models.IdentityClaims, organizationID uuid.UUID) (Token, error)
}

type Token struct {
	Credentials      models.Credentials
	Value            string
	Expiration       time.Time
	UsedSecondFactor bool
}

type MarbleTokenGenerator struct {
	repository marbleRepository

	clock         clock.Clock
	tokenLifetime time.Duration
	encoder       encoder
}

func NewGenerator(repository marbleRepository, encoder encoder, lifetime time.Duration, clock clock.Clock) TokenGenerator {
	return MarbleTokenGenerator{
		repository:    repository,
		encoder:       encoder,
		tokenLifetime: lifetime,
		clock:         clock,
	}
}

func (g MarbleTokenGenerator) GenerateToken(ctx context.Context, creds Credentials,
	intoCredentials models.IntoCredentials, claims models.IdentityClaims, requestedOrganizationID uuid.UUID,
) (Token, error) {
	expirationTime := g.clock.Now().Add(g.tokenLifetime)
	baseCredentials := intoCredentials.IntoCredentials()
	principalType, principalID := "user", string(baseCredentials.ActorIdentity.UserId)
	if creds.Type == CredentialsApiKey {
		principalType, principalID = "api_key", baseCredentials.ActorIdentity.ApiKeyId
	}
	grants, err := g.repository.ActiveGrantsForPrincipal(ctx, principalType, principalID)
	if err != nil {
		return Token{}, fmt.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}

	selectedOrganizationID := requestedOrganizationID
	if creds.Type == CredentialsApiKey {
		selectedOrganizationID = baseCredentials.OrganizationId
	} else if selectedOrganizationID == uuid.Nil {
		organizationIDs := []uuid.UUID{}
		hasPlatformGrant := false
		for _, grant := range grants {
			if grant.TenantId == uuid.Nil && grant.OrgId == uuid.Nil {
				hasPlatformGrant = true
			}
			if grant.OrgId != uuid.Nil && !slices.Contains(organizationIDs, grant.OrgId) {
				organizationIDs = append(organizationIDs, grant.OrgId)
			}
		}
		if !hasPlatformGrant && len(organizationIDs) == 1 {
			selectedOrganizationID = organizationIDs[0]
		}
	}

	selectedOrganization := models.Organization{}
	if selectedOrganizationID != uuid.Nil {
		selectedOrganization, err = g.repository.GetOrganizationByID(ctx, selectedOrganizationID)
		if err != nil {
			return Token{}, fmt.Errorf("GetOrganizationByID error: %w", err)
		}

		if len(models.ScopeRoleBindings(grants, selectedOrganizationID, selectedOrganization.TenantId)) == 0 {
			return Token{}, fmt.Errorf("%w: no access to organization", models.ForbiddenError)
		}
	}

	if len(grants) == 0 {
		return Token{}, fmt.Errorf("%w: principal has no active grant", models.ForbiddenError)
	}

	// Role bindings and permissions are informative here: they are resolved
	// again from grants on every authenticated request.
	tokenCredentials := baseCredentials
	tokenCredentials.OrganizationId = selectedOrganizationID
	tokenCredentials.RoleBindingBundle = models.RoleBindingBundle{
		UsedSecondFactor: claims.GetUsedSecondFactor(),
		ClientIp:         creds.ClientIp,
	}
	tokenCredentials.RoleBindings = models.ScopeRoleBindings(grants, selectedOrganizationID, selectedOrganization.TenantId)
	// Caveats are evaluated with the generator's clock, which is not kept in
	// the token's credentials.
	bundle := tokenCredentials.RoleBindingBundle
	bundle.Clock = g.clock
	tokenCredentials.Permissions = models.RoleBindingsPermissions(tokenCredentials.RoleBindings, bundle)

	switch creds.Type {
	case CredentialsBearer:
		if selectedOrganizationID != uuid.Nil {
			tracking.Identify(ctx, tokenCredentials.ActorIdentity.UserId, map[string]any{
				"email": tokenCredentials.ActorIdentity.Email,
			})
			tracking.Group(ctx, tokenCredentials.ActorIdentity.UserId,
				selectedOrganizationID, map[string]any{
					"name": selectedOrganization.Name,
				})
			tracking.TrackEventWithUserId(ctx, models.AnalyticsTokenCreated,
				tokenCredentials.ActorIdentity.UserId, map[string]any{
					"organization_id": selectedOrganizationID,
				})
		}
	}

	token, err := g.encoder.EncodeMarbleToken(claims.GetIssuer(), expirationTime, tokenCredentials)
	if err != nil {
		return Token{}, fmt.Errorf("encoder.EncodeMarbleToken error: %w", err)
	}

	return Token{tokenCredentials, token, expirationTime, claims.GetUsedSecondFactor()}, nil
}
