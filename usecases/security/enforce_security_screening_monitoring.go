package security

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityScreeningMonitoring interface {
	EnforceSecurity
	ReadScreeningMonitoringConfig(ctx context.Context, config models.ScreeningMonitoringConfig) error
	WriteScreeningMonitoringConfig(ctx context.Context, orgId string) error
	WriteScreeningMonitoringObject(ctx context.Context, orgId string) error
}

type EnforceSecurityScreeningMonitoringImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityScreeningMonitoringImpl) ReadScreeningMonitoringConfig(ctx context.Context, config models.ScreeningMonitoringConfig) error {
	return errors.Join(
		e.Permission(models.SCREENING_MONITORING_READ),
		e.ReadOrganization(config.OrgId),
	)
}

func (e *EnforceSecurityScreeningMonitoringImpl) WriteScreeningMonitoringConfig(ctx context.Context, orgId string) error {
	return errors.Join(
		e.Permission(models.SCREENING_MONITORING_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityScreeningMonitoringImpl) WriteScreeningMonitoringObject(ctx context.Context, orgId string) error {
	return errors.Join(
		e.Permission(models.SCREENING_MONITORING_WRITE),
		e.ReadOrganization(orgId),
	)
}
