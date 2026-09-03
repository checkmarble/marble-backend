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
	ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.Grant, error)
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
	Credentials models.Credentials
	Value       string
	Expiration  time.Time
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
	// Roles are the union of the legacy role (users.role / api_keys.role) and the
	// active grants, until the authority is switched to grants-only together with
	// the backfill. The legacy role only enters a token of its own scope.
	legacyOrganizationID := baseCredentials.OrganizationId
	legacyRole := baseCredentials.Role

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
		selectedOrganizationID = legacyOrganizationID
	} else if selectedOrganizationID == uuid.Nil {
		organizationIDs := []uuid.UUID{}
		if legacyOrganizationID != uuid.Nil {
			organizationIDs = append(organizationIDs, legacyOrganizationID)
		}
		for _, grant := range grants {
			if grant.OrganizationId != uuid.Nil && !slices.Contains(organizationIDs, grant.OrganizationId) {
				organizationIDs = append(organizationIDs, grant.OrganizationId)
			}
		}
		if len(organizationIDs) == 1 {
			selectedOrganizationID = organizationIDs[0]
		}
	}

	selectedOrganization := models.Organization{}
	if selectedOrganizationID != uuid.Nil {
		selectedOrganization, err = g.repository.GetOrganizationByID(ctx, selectedOrganizationID)
		if err != nil {
			return Token{}, fmt.Errorf("GetOrganizationByID error: %w", err)
		}

		legacyOrganizationAccess := selectedOrganizationID == legacyOrganizationID &&
			legacyRole != models.NO_ROLE && legacyRole != models.MARBLE_ADMIN
		grantOrganizationAccess := false
		for _, grant := range grants {
			if grant.OrganizationId == selectedOrganizationID || grant.TenantId == selectedOrganization.TenantId {
				grantOrganizationAccess = true
				break
			}
		}
		if !legacyOrganizationAccess && !grantOrganizationAccess {
			return Token{}, fmt.Errorf("%w: no access to organization", models.ForbiddenError)
		}
	}

	tokenCredentials := baseCredentials
	tokenCredentials.OrganizationId = selectedOrganizationID
	tokenCredentials.TenantId = selectedOrganization.TenantId

	roles := []models.Role{}
	addRole := func(role models.Role) {
		if !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}
	if selectedOrganizationID == uuid.Nil {
		if legacyRole == models.MARBLE_ADMIN {
			addRole(legacyRole)
		}
		for _, grant := range grants {
			if grant.TenantId == uuid.Nil && grant.OrganizationId == uuid.Nil {
				addRole(grant.Role)
			}
		}
	} else {
		if legacyRole != models.NO_ROLE && legacyRole != models.MARBLE_ADMIN &&
			legacyOrganizationID == selectedOrganizationID {
			addRole(legacyRole)
		}
		for _, grant := range grants {
			if grant.OrganizationId == selectedOrganizationID || grant.TenantId == selectedOrganization.TenantId {
				addRole(grant.Role)
			}
		}
	}
	tokenCredentials.Roles = roles
	tokenCredentials.Role = legacyRole

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

	return Token{tokenCredentials, token, expirationTime}, nil
}
