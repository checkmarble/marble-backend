package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityScreeningMonitoring interface {
	EnforceSecurity
	ReadScreeningMonitoringConfig(config models.ScreeningMonitoringConfig) error
	WriteScreeningMonitoringConfig(orgId string) error
}

type EnforceSecurityScreeningMonitoringImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScreeningMonitoringImpl) ReadScreeningMonitoringConfig(config models.ScreeningMonitoringConfig) error {
	return errors.Join(
		e.Permission(models.SCREENING_MONITORING_READ),
		e.ReadOrganization(config.OrgId),
	)
}

func (e *EnforceSecurityScreeningMonitoringImpl) WriteScreeningMonitoringConfig(orgId string) error {
	return errors.Join(
		e.Permission(models.SCREENING_MONITORING_WRITE),
		e.ReadOrganization(orgId),
	)
}
