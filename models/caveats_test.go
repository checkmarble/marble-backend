package models

import (
	"encoding/json"
	"net"
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

func TestRoleBindingNetworksCaveat(t *testing.T) {
	subnet := func(cidr string) Subnet {
		s, err := ParseSubnet(cidr)
		assert.NoError(t, err)
		return s
	}

	restricted := RoleBinding{Conditions: RoleBindingConditions{
		Networks: []Subnet{subnet("10.0.0.0/8"), subnet("2001:db8::/32")},
	}}

	tests := []struct {
		name     string
		binding  RoleBinding
		clientIp net.IP
		active   bool
	}{
		{"ipv4 inside", restricted, net.ParseIP("10.1.2.3"), true},
		{"ipv6 inside", restricted, net.ParseIP("2001:db8::1"), true},
		{"ipv4-mapped ipv6 inside", restricted, net.ParseIP("::ffff:10.1.2.3"), true},
		{"outside", restricted, net.ParseIP("192.168.1.1"), false},
		{"unknown client IP fails open", restricted, nil, true},
		{"empty list is unrestricted", RoleBinding{Conditions: RoleBindingConditions{Networks: []Subnet{}}}, net.ParseIP("192.168.1.1"), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.active, test.binding.IsActive(RoleBindingBundle{ClientIp: test.clientIp}))
		})
	}
}

func TestRoleBindingNetworksDecoding(t *testing.T) {
	var conditions RoleBindingConditions

	assert.NoError(t, json.Unmarshal([]byte(`{"networks":["10.0.0.1","10.1.0.0/16"]}`), &conditions))
	assert.Equal(t, "10.0.0.1/32", conditions.Networks[0].String())
	assert.Equal(t, "10.1.0.0/16", conditions.Networks[1].String())

	encoded, err := json.Marshal(conditions)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"networks":["10.0.0.1/32","10.1.0.0/16"]}`, string(encoded))

	assert.Error(t, json.Unmarshal([]byte(`{"networks":["nope"]}`), &conditions))
	assert.Error(t, json.Unmarshal([]byte(`{"networks":["10.0.0.0/33"]}`), &conditions))
}

func TestCredentialsEvaluateCaveatsAgainstTheirBundle(t *testing.T) {
	required := true
	office, err := ParseSubnet("10.0.0.0/8")
	assert.NoError(t, err)

	binding := NewNativeRoleBinding(ADMIN)
	binding.Conditions = RoleBindingConditions{
		UsedSecondFactor: &required,
		Networks:         []Subnet{office},
	}

	tests := []struct {
		name   string
		bundle RoleBindingBundle
		active bool
	}{
		{"second factor from the office", RoleBindingBundle{UsedSecondFactor: true, ClientIp: net.ParseIP("10.1.2.3")}, true},
		{"no second factor", RoleBindingBundle{UsedSecondFactor: false, ClientIp: net.ParseIP("10.1.2.3")}, false},
		{"outside the office", RoleBindingBundle{UsedSecondFactor: true, ClientIp: net.ParseIP("192.168.1.1")}, false},
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
