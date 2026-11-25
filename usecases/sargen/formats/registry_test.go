package formats_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/checkmarble/marble-backend/usecases/sargen/formats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_GetTracfin(t *testing.T) {
	key, err := core.ParseFormatKey("tracfin:FR:1.4.0")
	require.NoError(t, err)

	f, ok := formats.Get(key)
	require.True(t, ok)
	assert.Equal(t, key.String(), f.Key().String())

	goamlKey, err := core.ParseFormatKey("goaml:LU:5.0.2")
	require.NoError(t, err)
	_, ok = formats.Get(goamlKey)
	assert.False(t, ok)
}

func TestRegistry_ListContainsTracfin(t *testing.T) {
	var keys []string
	for _, f := range formats.List() {
		keys = append(keys, f.Key().String())
	}
	assert.Contains(t, keys, "tracfin:FR:1.4.0")
}
