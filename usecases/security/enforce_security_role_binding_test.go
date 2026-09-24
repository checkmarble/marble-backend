package security

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/stretchr/testify/assert"
)

func TestPermissionEvaluatesRoleBindingWindow(t *testing.T) {
	now := time.Date(2026, time.September, 24, 10, 0, 0, 0, time.UTC)
	before := now.Add(-time.Minute)
	after := now.Add(time.Minute)

	tests := []struct {
		name      string
		notBefore *time.Time
		notAfter  *time.Time
		allowed   bool
	}{
		{name: "unconditional", allowed: true},
		{name: "notBefore is inclusive", notBefore: &now, allowed: true},
		{name: "notAfter is exclusive", notAfter: &now, allowed: false},
		{name: "active window", notBefore: &before, notAfter: &after, allowed: true},
		{name: "future window", notBefore: &after, allowed: false},
		{name: "expired window", notAfter: &before, allowed: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enforcer := EnforceSecurityImpl{
				Clock: clock.NewMock(now),
				Credentials: models.Credentials{RoleBindings: []models.RoleBinding{{
					Permissions: []models.Permission{models.DECISION_READ},
					Conditions: models.RoleBindingConditions{
						NotBefore: test.notBefore,
						NotAfter:  test.notAfter,
					},
				}}},
			}

			err := enforcer.Permission(models.DECISION_READ)
			if test.allowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, models.ForbiddenError)
			}
		})
	}
}

func TestPermissionAllowsAnyActiveBinding(t *testing.T) {
	now := time.Date(2026, time.September, 24, 10, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)
	role := models.VIEWER

	enforcer := EnforceSecurityImpl{
		Clock: clock.NewMock(now),
		Credentials: models.Credentials{RoleBindings: []models.RoleBinding{
			{
				Role:        role,
				Permissions: []models.Permission{models.DECISION_READ},
				Conditions:  models.RoleBindingConditions{NotAfter: &expired},
			},
			{
				Role:        role,
				Permissions: []models.Permission{models.DECISION_READ},
			},
		}},
	}

	assert.NoError(t, enforcer.Permission(models.DECISION_READ))
}
