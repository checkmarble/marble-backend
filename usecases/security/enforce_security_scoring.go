package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityScoring interface {
	EnforceSecurity

	ReadRecordScore(models.ScoringScore) error
	UpdateSettings(uuid.UUID) error
	UpdateRuleset(uuid.UUID) error
	OverrideScore(models.ScoringRecordRef) error
}

type EnforceSecurityScoringImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScoringImpl) ReadRecordScore(score models.ScoringScore) error {
	return e.ReadOrganization(score.OrgId)
}

func (e *EnforceSecurityScoringImpl) UpdateSettings(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.SCORING_UPDATE_SETTINGS),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScoringImpl) UpdateRuleset(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.SCORING_UPDATE_RULESETS),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScoringImpl) OverrideScore(record models.ScoringRecordRef) error {
	return errors.Join(
		e.Permission(models.SCORING_OVERRIDE_SCORE),
		e.ReadOrganization(record.OrgId),
	)
}
