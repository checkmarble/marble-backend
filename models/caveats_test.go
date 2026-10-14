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

func TestRoleBindingWeekDaysCaveat(t *testing.T) {
	// 2026-09-28 is a Monday.
	monday := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		weekDays *[]time.Weekday
		now      time.Time
		active   bool
	}{
		{"unrestricted", nil, monday, true},
		{"allowed day", &[]time.Weekday{time.Monday, time.Tuesday}, monday, true},
		{"other day", &[]time.Weekday{time.Tuesday}, monday, false},
		{"weekend only", &[]time.Weekday{time.Saturday, time.Sunday}, monday.AddDate(0, 0, 5), true},
		{"empty list is unrestricted", &[]time.Weekday{}, monday, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := RoleBinding{Conditions: RoleBindingConditions{DayOfWeek: test.weekDays}}
			assert.Equal(t, test.active, binding.IsActive(RoleBindingBundle{Clock: clock.NewMock(test.now)}))
		})
	}
}

func TestRoleBindingWeekDaysTimeZone(t *testing.T) {
	// 2026-09-28 23:30 UTC is Monday in UTC, but already Tuesday in Paris.
	now := time.Date(2026, 9, 28, 23, 30, 0, 0, time.UTC)
	paris := time.FixedZone("Europe/Paris", 2*60*60)
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)

	mondayOnly := RoleBinding{Conditions: RoleBindingConditions{DayOfWeek: &[]time.Weekday{time.Monday}}}

	tests := []struct {
		name     string
		now      time.Time
		location *time.Location
		active   bool
	}{
		{"organization in UTC", now, time.UTC, true},
		{"organization in Paris", now, paris, false},
		{"no time zone falls back to UTC", now, nil, true},
		{"no time zone ignores the evaluation time's zone", now.In(tokyo), nil, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := RoleBindingBundle{Clock: clock.NewMock(test.now), Location: test.location}
			assert.Equal(t, test.active, mondayOnly.IsActive(bundle))
		})
	}
}

func TestOrganizationTimeZone(t *testing.T) {
	zone := func(name string) *string { return &name }

	assert.Equal(t, time.UTC, Organization{}.Timezone())
	assert.Equal(t, time.UTC, Organization{DefaultScenarioTimezone: zone("")}.Timezone())
	assert.Equal(t, time.UTC, Organization{DefaultScenarioTimezone: zone("Mars/Olympus_Mons")}.Timezone())
	assert.Equal(t, "Europe/Paris", Organization{DefaultScenarioTimezone: zone("Europe/Paris")}.Timezone().String())
}

func TestRoleBindingWeekDaysDecoding(t *testing.T) {
	var conditions RoleBindingConditions

	assert.NoError(t, json.Unmarshal([]byte(`{"dayOfWeek":[1,5]}`), &conditions))
	assert.Equal(t, []time.Weekday{time.Monday, time.Friday}, *conditions.DayOfWeek)

	encoded, err := json.Marshal(conditions)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"dayOfWeek":[1,5]}`, string(encoded))

	assert.Error(t, json.Unmarshal([]byte(`{"dayOfWeek":["monday"]}`), &conditions))
}

func TestRoleBindingTimeOfDayCaveat(t *testing.T) {
	at := func(hour, minute int) time.Time {
		return time.Date(2026, 9, 28, hour, minute, 0, 0, time.UTC)
	}
	timeOfDay := func(start, end int) *TimeOfDayRange {
		r, err := NewTimeOfDayRange(start, end)
		assert.NoError(t, err)
		return &r
	}

	officeHours := timeOfDay(900, 1750)
	nightShift := timeOfDay(2200, 600)

	tests := []struct {
		name      string
		timeOfDay *TimeOfDayRange
		now       time.Time
		active    bool
	}{
		{"unrestricted", nil, at(3, 0), true},
		{"within office hours", officeHours, at(12, 30), true},
		{"at the start, inclusive", officeHours, at(9, 0), true},
		{"at the end, inclusive", officeHours, at(17, 50), true},
		{"just after the end", officeHours, at(17, 51), false},
		{"before office hours", officeHours, at(8, 59), false},
		{"after office hours", officeHours, at(19, 50), false},
		{"night shift, before midnight", nightShift, at(23, 0), true},
		{"night shift, at midnight", nightShift, at(0, 0), true},
		{"night shift, after midnight", nightShift, at(5, 59), true},
		{"night shift, at the end", nightShift, at(6, 0), true},
		{"night shift, just after the end", nightShift, at(6, 1), false},
		{"outside the night shift", nightShift, at(12, 0), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding := RoleBinding{Conditions: RoleBindingConditions{TimeOfDay: test.timeOfDay}}
			assert.Equal(t, test.active, binding.IsActive(RoleBindingBundle{Clock: clock.NewMock(test.now)}))
		})
	}
}

func TestRoleBindingTimeOfDayTimeZone(t *testing.T) {
	// 07:30 UTC is 09:30 in Paris.
	now := time.Date(2026, 9, 28, 7, 30, 0, 0, time.UTC)
	paris := time.FixedZone("Europe/Paris", 2*60*60)
	tokyo := time.FixedZone("Asia/Tokyo", 9*60*60)

	officeHours := RoleBinding{Conditions: RoleBindingConditions{TimeOfDay: &TimeOfDayRange{900, 1750}}}

	tests := []struct {
		name     string
		now      time.Time
		location *time.Location
		active   bool
	}{
		{"organization in UTC", now, time.UTC, false},
		{"organization in Paris", now, paris, true},
		{"no time zone falls back to UTC", now, nil, false},
		{"no time zone ignores the evaluation time's zone", now.In(tokyo), nil, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := RoleBindingBundle{Clock: clock.NewMock(test.now), Location: test.location}
			assert.Equal(t, test.active, officeHours.IsActive(bundle))
		})
	}
}

func TestRoleBindingTimeOfDayValidation(t *testing.T) {
	valid := [][2]int{{0, 2359}, {900, 1750}, {2200, 600}, {1950, 0}, {900, 900}}
	invalid := [][2]int{{-1, 900}, {900, 2400}, {960, 1700}, {900, 1760}, {2500, 100}}

	for _, times := range valid {
		_, err := NewTimeOfDayRange(times[0], times[1])
		assert.NoError(t, err, times)
		assert.NoError(t, RoleBindingConditions{TimeOfDay: &TimeOfDayRange{times[0], times[1]}}.Validate(), times)
	}

	for _, times := range invalid {
		_, err := NewTimeOfDayRange(times[0], times[1])
		assert.ErrorIs(t, err, BadParameterError, times)
		assert.ErrorIs(t, RoleBindingConditions{TimeOfDay: &TimeOfDayRange{times[0], times[1]}}.Validate(), BadParameterError, times)
	}
}

func TestRoleBindingTimeOfDayDecoding(t *testing.T) {
	var conditions RoleBindingConditions

	assert.NoError(t, json.Unmarshal([]byte(`{"timeOfDay":[900,1950]}`), &conditions))
	assert.Equal(t, TimeOfDayRange{900, 1950}, *conditions.TimeOfDay)

	encoded, err := json.Marshal(conditions)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"timeOfDay":[900,1950]}`, string(encoded))

	for _, input := range []string{
		`{"timeOfDay":[900]}`,
		`{"timeOfDay":[900,1200,1700]}`,
		`{"timeOfDay":[900,1960]}`,
		`{"timeOfDay":["9:00","17:50"]}`,
		`{"timeOfDay":900}`,
	} {
		assert.Error(t, json.Unmarshal([]byte(input), &RoleBindingConditions{}), input)
	}
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
