package security

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityScreening interface {
	EnforceSecurity

	ReadWhitelist(ctx context.Context) error
	WriteWhitelist(ctx context.Context) error
	PerformFreeformSearch(ctx context.Context) error
	ReadFreeformSearch(s models.FreeformSearch) error
	SaveFreeformSearch(s models.FreeformSearch) error
}

func (e *EnforceSecurityImpl) ReadWhitelist(ctx context.Context) error {
	return e.Permission(models.SCREENING_WHITELIST_READ)
}

func (e *EnforceSecurityImpl) WriteWhitelist(ctx context.Context) error {
	return e.Permission(models.SCREENING_WHITELIST_WRITE)
}

func (e *EnforceSecurityImpl) PerformFreeformSearch(ctx context.Context) error {
	return e.Permission(models.SCREENING_FREEFORM_SEARCH)
}

func (e *EnforceSecurityImpl) ReadFreeformSearch(s models.FreeformSearch) error {
	return errors.Join(
		e.Permission(models.SCREENING_FREEFORM_SEARCH),
		e.ReadOrganization(s.OrgId),
	)
}

// SaveFreeformSearch only allows the actor who performed the search to save its results. We match
// on the user id (or api key id for API clients) that was recorded when the search was performed.
func (e *EnforceSecurityImpl) SaveFreeformSearch(s models.FreeformSearch) error {
	if err := errors.Join(
		e.Permission(models.SCREENING_FREEFORM_SEARCH),
		e.ReadOrganization(s.OrgId),
	); err != nil {
		return err
	}

	if userId := e.UserId(); userId != nil {
		if s.UserId != nil && s.UserId.String() == *userId {
			return nil
		}
	}

	if apiKeyId := e.ApiKeyId(); apiKeyId != nil {
		if s.ApiKeyId != nil && s.ApiKeyId.String() == *apiKeyId {
			return nil
		}
	}

	return errors.Wrap(models.ForbiddenError, "freeform search can only be saved by the actor who performed it")
}
