package auth

import (
	"context"
	"fmt"
	"slices"
	"sort"
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
	intoCredentials models.IntoCredentials, claims models.IdentityClaims, organizationID uuid.UUID,
) (Token, error) {
	expirationTime := g.clock.Now().Add(g.tokenLifetime)
	credentials := intoCredentials.IntoCredentials()
	principalType, principalID := "user", string(credentials.ActorIdentity.UserId)
	if creds.Type == CredentialsApiKey {
		principalType, principalID = "api_key", credentials.ActorIdentity.ApiKeyId
	} else {
		credentials.OrganizationId = uuid.Nil
		credentials.TenantId = uuid.Nil
	}
	grants, err := g.repository.ActiveGrantsForPrincipal(ctx, principalType, principalID)
	if err != nil {
		return Token{}, fmt.Errorf("ActiveGrantsForPrincipal error: %w", err)
	}

	if creds.Type == CredentialsApiKey {
		organizationID = credentials.OrganizationId
	} else if organizationID == uuid.Nil {
		organizationIDs := map[uuid.UUID]struct{}{}
		for _, grant := range grants {
			if grant.OrganizationId != uuid.Nil {
				organizationIDs[grant.OrganizationId] = struct{}{}
			}
		}
		if len(organizationIDs) == 1 {
			for organizationID = range organizationIDs {
			}
		}
	}
	if organizationID != uuid.Nil {
		organization, err := g.repository.GetOrganizationByID(ctx, organizationID)
		if err != nil {
			return Token{}, fmt.Errorf("GetOrganizationByID error: %w", err)
		}
		hasOrganizationGrant := false
		for _, grant := range grants {
			if grant.OrganizationId == organizationID || grant.TenantId == organization.TenantId {
				hasOrganizationGrant = true
				break
			}
		}
		if !hasOrganizationGrant {
			return Token{}, fmt.Errorf("%w: no access to organization", models.ForbiddenError)
		}
		credentials.OrganizationId = organizationID
		credentials.TenantId = organization.TenantId
	}

	roles := []models.Role{}
	addRole := func(role models.Role) {
		if !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}
	if credentials.OrganizationId == uuid.Nil {
		for _, grant := range grants {
			if grant.TenantId == uuid.Nil && grant.OrganizationId == uuid.Nil {
				addRole(grant.Role)
			}
		}
	} else {
		for _, grant := range grants {
			if grant.OrganizationId == credentials.OrganizationId || grant.TenantId == credentials.TenantId {
				addRole(grant.Role)
			}
		}
	}
	credentials.Roles = roles
	sort.Slice(credentials.Roles, func(i, j int) bool { return credentials.Roles[i] < credentials.Roles[j] })
	if len(credentials.Roles) == 0 {
		return Token{}, fmt.Errorf("%w: principal has no active grant", models.ForbiddenError)
	}

	switch creds.Type {
	case CredentialsBearer:
		if credentials.OrganizationId != uuid.Nil {
			tracking.Identify(ctx, credentials.ActorIdentity.UserId, map[string]any{
				"email": credentials.ActorIdentity.Email,
			})
			tracking.Group(ctx, credentials.ActorIdentity.UserId,
				credentials.OrganizationId, map[string]any{
					"name": credentials.OrganizationId.String(),
				})
			tracking.TrackEventWithUserId(ctx, models.AnalyticsTokenCreated,
				credentials.ActorIdentity.UserId, map[string]any{
					"organization_id": credentials.OrganizationId,
				})
		}
	}

	token, err := g.encoder.EncodeMarbleToken(claims.GetIssuer(), expirationTime, credentials)
	if err != nil {
		return Token{}, fmt.Errorf("encoder.EncodeMarbleToken error: %w", err)
	}

	return Token{credentials, token, expirationTime}, nil
}
