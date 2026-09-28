package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRoleBindingsSections(t *testing.T) {
	expired := time.Now().Add(-time.Hour)

	for _, tt := range []struct {
		name     string
		bindings []RoleBinding
		hidden   []AppSection
	}{
		{name: "no bindings"},
		{name: "unrestricted", bindings: NativeRoleBindings([]Role{ADMIN})},
		{name: "analyst", bindings: NativeRoleBindings([]Role{ANALYST}), hidden: ANALYST.HiddenSections()},
		{name: "unrestricted binding wins", bindings: NativeRoleBindings([]Role{ANALYST, VIEWER})},
		{name: "custom intersection", bindings: []RoleBinding{
			{HiddenSections: []AppSection{SCENARIOS, SETTINGS}},
			{HiddenSections: []AppSection{SCENARIOS, DATA_MODEL}},
		}, hidden: []AppSection{SCENARIOS}},
		{name: "inactive binding ignored", bindings: []RoleBinding{
			{HiddenSections: []AppSection{SETTINGS}},
			{Conditions: RoleBindingConditions{NotAfter: &expired}},
		}, hidden: []AppSection{SETTINGS}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, SectionsVisibility(tt.hidden), RoleBindingsSections(tt.bindings, RoleBindingBundle{}))
		})
	}
}
