package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOrganizationShortId(t *testing.T) {
	aa := NewOrganizationShortId("12345678-ffff-ffff-ffff-ffffffffffff")
	assert.Equal(t, aa.String(), "12345678")
}
