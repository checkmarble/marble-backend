package repositories

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func TestSameRoleBinding(t *testing.T) {
	notBefore := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	sameInstantInParis := notBefore.In(time.FixedZone("Europe/Paris", 2*60*60))
	later := notBefore.Add(time.Hour)
	required := true
	notRequired := false

	binding := func(conditions models.RoleBindingConditions) models.RoleBinding {
		return models.RoleBinding{Role: models.VIEWER, Conditions: conditions}
	}

	current := binding(models.RoleBindingConditions{
		NotBefore:        &notBefore,
		UsedSecondFactor: &required,
	})

	with := func(change func(c *models.RoleBindingConditions)) models.RoleBinding {
		next := current
		change(&next.Conditions)
		return next
	}

	tests := []struct {
		name string
		next models.RoleBinding
		same bool
	}{
		{"identical", current, true},
		{"same instant in another time zone", with(func(c *models.RoleBindingConditions) { c.NotBefore = &sameInstantInParis }), true},
		{"other role", models.RoleBinding{Role: models.ADMIN, Conditions: current.Conditions}, false},
		{"not before changed", with(func(c *models.RoleBindingConditions) { c.NotBefore = &later }), false},
		{"not after added", with(func(c *models.RoleBindingConditions) { c.NotAfter = &later }), false},
		{"second factor changed", with(func(c *models.RoleBindingConditions) { c.UsedSecondFactor = &notRequired }), false},
		{"all conditions removed", binding(models.RoleBindingConditions{}), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.same, sameRoleBinding(current, test.next))
		})
	}
}
