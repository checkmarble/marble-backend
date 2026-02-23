package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityScoring interface {
	EnforceSecurity

	ReadEntityScore(models.ScoringScore) error
	OverrideScore(models.ScoringEntityRef) error
}

type EnforceSecurityScoringImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScoringImpl) ReadEntityScore(score models.ScoringScore) error {
	return e.ReadOrganization(score.OrgId)
}

func (e *EnforceSecurityScoringImpl) OverrideScore(entityRef models.ScoringEntityRef) error {
	return errors.Join(
		e.Permission(models.SCORING_OVERRIDE_SCORE),
		e.ReadOrganization(entityRef.OrgId),
	)
}
