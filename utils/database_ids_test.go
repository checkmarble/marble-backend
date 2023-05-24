package utils

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPrimaryKey(t *testing.T) {
	orgId := "86d9b92d-e654-4de3-8d3f-81830246c891"

	newId := NewPrimaryKey(orgId)

	log.Println(orgId)
	log.Println(newId)

	asserts := assert.New(t)
	asserts.Equal(orgId[:8], newId[:8])
	asserts.NotEqual(orgId, newId)

}

func TestNewUUIDStartWithOrgId(t *testing.T) {
	newId := NewPrimaryKey("12345678-ffff-ffff-ffff-ffffffffffff")

	// first 8 characters are the org id
	assert.Equal(t, newId[:8], "12345678")
	// the rest is diffenrent
	assert.NotEqual(t, newId[8:], "-ffff-ffff-ffff-ffffffffffff")
}
