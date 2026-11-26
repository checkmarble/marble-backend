package tracfin

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatus(t *testing.T) {
	for raw, want := range map[string]core.VersionStatus{
		`{"status":"active"}`:     core.VersionActive,
		`{"status":"deprecated"}`: core.VersionDeprecated,
		`{"status":"retired"}`:    core.VersionRetired,
	} {
		got, err := parseStatus([]byte(raw))
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
	_, err := parseStatus([]byte(`{"status":"nope"}`))
	assert.Error(t, err)
	_, err = parseStatus([]byte(`{`))
	assert.Error(t, err)
}
