package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/stretchr/testify/assert"
)

func TestRoleBindingValidityCaveat(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	before := now.Add(-time.Hour)
	after := now.Add(time.Hour)

	tests := []struct {
		name       string
		conditions RoleBindingConditions
		active     bool
	}{
		{"no conditions", RoleBindingConditions{}, true},
		{"not yet valid", RoleBindingConditions{NotBefore: &after}, false},
		{"valid since", RoleBindingConditions{NotBefore: &before}, true},
		{"valid from exactly now", RoleBindingConditions{NotBefore: &now}, true},
		{"expired", RoleBindingConditions{NotAfter: &before}, false},
		{"expires exactly now, inclusive", RoleBindingConditions{NotAfter: &now}, true},
		{"not expired yet", RoleBindingConditions{NotAfter: &after}, true},
		{"within window", RoleBindingConditions{NotBefore: &before, NotAfter: &after}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := RoleBinding{Conditions: test.conditions}
			assert.Equal(t, test.active, binding.IsActive(RoleBindingBundle{Clock: clock.NewMock(now)}))
		})
	}
}

func TestRoleBindingValidityValidation(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)

	assert.NoError(t, RoleBindingConditions{}.Validate())
	assert.NoError(t, RoleBindingConditions{NotBefore: &now}.Validate())
	assert.NoError(t, RoleBindingConditions{NotAfter: &now}.Validate())
	assert.NoError(t, RoleBindingConditions{NotBefore: &now, NotAfter: &later}.Validate())
	assert.ErrorIs(t, RoleBindingConditions{NotBefore: &later, NotAfter: &now}.Validate(), BadParameterError)
	assert.ErrorIs(t, RoleBindingConditions{NotBefore: &now, NotAfter: &now}.Validate(), BadParameterError)
}

func TestRoleBindingValidityDecoding(t *testing.T) {
	var conditions RoleBindingConditions

	assert.NoError(t, json.Unmarshal([]byte(`{"notBefore":"2026-09-28T11:00:00Z","notAfter":"2026-09-28T13:00:00+02:00"}`), &conditions))
	assert.True(t, conditions.NotBefore.Equal(time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC)))
	assert.True(t, conditions.NotAfter.Equal(time.Date(2026, 9, 28, 11, 0, 0, 0, time.UTC)))

	encoded, err := json.Marshal(RoleBindingConditions{})
	assert.NoError(t, err)
	assert.JSONEq(t, `{}`, string(encoded))

	assert.Error(t, json.Unmarshal([]byte(`{"notBefore":"yesterday"}`), &conditions))
}
func TestRoleBindingSecondFactorCaveat(t *testing.T) {
	required, notRequired := true, false

	tests := []struct {
		name             string
		requirement      *bool
		usedSecondFactor bool
		active           bool
	}{
		{"required and used", &required, true, true},
		{"required but not used", &required, false, false},
		{"explicitly not required", &notRequired, false, true},
		{"unrestricted", nil, false, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := RoleBinding{Conditions: RoleBindingConditions{UsedSecondFactor: test.requirement}}
			bundle := RoleBindingBundle{UsedSecondFactor: test.usedSecondFactor}
			assert.Equal(t, test.active, binding.IsActive(bundle))
		})
	}
}

func TestRoleBindingSecondFactorDecoding(t *testing.T) {
	var conditions RoleBindingConditions

	assert.NoError(t, json.Unmarshal([]byte(`{"usedSecondFactor":true}`), &conditions))
	assert.True(t, *conditions.UsedSecondFactor)

	encoded, err := json.Marshal(conditions)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"usedSecondFactor":true}`, string(encoded))

	assert.Error(t, json.Unmarshal([]byte(`{"usedSecondFactor":"yes"}`), &conditions))
}
func TestCredentialsEvaluateCaveatsAgainstTheirBundle(t *testing.T) {
	required := true

	binding := NewNativeRoleBinding(ADMIN)
	binding.Conditions = RoleBindingConditions{UsedSecondFactor: &required}

	tests := []struct {
		name   string
		bundle RoleBindingBundle
		active bool
	}{
		{"second factor", RoleBindingBundle{UsedSecondFactor: true}, true},
		{"no second factor", RoleBindingBundle{UsedSecondFactor: false}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			credentials := Credentials{RoleBindings: []RoleBinding{binding}, RoleBindingBundle: test.bundle}

			assert.Equal(t, test.active, credentials.HasRole(ADMIN))
			assert.Equal(t, test.active, credentials.HasPermission(APIKEY_CREATE))
		})
	}
}

func TestCredentialsEvaluateCaveatsWithTheirClock(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	notAfter := now.Add(time.Hour)

	binding := NewNativeRoleBinding(ADMIN)
	binding.Conditions = RoleBindingConditions{NotAfter: &notAfter}

	at := func(t time.Time) Credentials {
		return Credentials{
			RoleBindings:      []RoleBinding{binding},
			RoleBindingBundle: RoleBindingBundle{Clock: clock.NewMock(t)},
		}
	}

	assert.True(t, at(now).HasRole(ADMIN))
	assert.True(t, at(now).HasPermission(APIKEY_CREATE))
	assert.False(t, at(notAfter.Add(time.Minute)).HasRole(ADMIN))
	assert.False(t, at(notAfter.Add(time.Minute)).HasPermission(APIKEY_CREATE))
}
