package core_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/usecases/sargen/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePath_RoundTripAndTemplate(t *testing.T) {
	p, err := core.ParsePath("personnes.pp[pp-1].etatCivil.nationalite[n-2]")
	require.NoError(t, err)
	assert.Equal(t, "personnes.pp[].etatCivil.nationalite[]", p.Template())
	assert.True(t, p.HasPrefix("personnes.pp[pp-1]"))
	assert.False(t, core.Path("personnes.pp[pp-10].x").HasPrefix("personnes.pp[pp-1]"))
	_, err = core.ParsePath("personnes.pp[PP 1]")
	assert.Error(t, err)
}

func TestParsePath_RejectsEmptySegments(t *testing.T) {
	for _, s := range []string{"a..b", ".a", "a."} {
		_, err := core.ParsePath(s)
		assert.Error(t, err, "expected error for %q", s)
	}
}

func TestParsePath_ProvisionalKey(t *testing.T) {
	p, err := core.ParsePath("x[+a]")
	require.NoError(t, err)
	assert.True(t, p.HasProvisionalKey())

	stable, err := core.ParsePath("personnes.pp[pp-1]")
	require.NoError(t, err)
	assert.False(t, stable.HasProvisionalKey())
}

func TestPath_ParentChildItem(t *testing.T) {
	p, err := core.ParsePath("a.b[k].c")
	require.NoError(t, err)
	assert.Equal(t, core.Path("a.b[k]"), p.Parent())
	assert.Equal(t, core.Path("a.b"), core.Path("a").Child("b"))
	assert.Equal(t, core.Path("personnes.pp[pp-1]"), core.Path("personnes").Item("pp", "pp-1"))
}

func TestFormatKey_RoundTrip(t *testing.T) {
	k, err := core.ParseFormatKey("tracfin:FR:1.4.0")
	require.NoError(t, err)
	assert.Equal(t, "tracfin", k.Family)
	assert.Equal(t, "FR", k.Country)
	assert.Equal(t, "1.4.0", k.Version)
	assert.Equal(t, "tracfin:FR:1.4.0", k.String())
}
