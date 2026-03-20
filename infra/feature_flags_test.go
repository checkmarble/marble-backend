package infra

import (
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/stretchr/testify/assert"
)

func setFeatureFlagEnv(t *testing.T, flag featureFlag, value string) {
	t.Helper()
	t.Setenv(fmt.Sprintf("ENABLE_%s", string(flag)), value)
}

func TestHasGlobalFeatureFlag_NotSet(t *testing.T) {
	t.Setenv(fmt.Sprintf("ENABLE_%s", string(FEATURE_USER_SCORING)), "")

	assert.False(t, HasGlobalFeatureFlag(FEATURE_USER_SCORING))
}

func TestHasGlobalFeatureFlag_Set(t *testing.T) {
	setFeatureFlagEnv(t, FEATURE_USER_SCORING, "all")

	assert.True(t, HasGlobalFeatureFlag(FEATURE_USER_SCORING))
}

func TestHasFeatureFlag_EnvNotSet(t *testing.T) {
	t.Setenv(fmt.Sprintf("ENABLE_%s", string(FEATURE_USER_SCORING)), "")

	orgId := pure_utils.NewId()

	assert.False(t, HasFeatureFlag(FEATURE_USER_SCORING, orgId))
}

func TestHasFeatureFlag_All(t *testing.T) {
	setFeatureFlagEnv(t, FEATURE_USER_SCORING, "all")

	assert.True(t, HasFeatureFlag(FEATURE_USER_SCORING, pure_utils.NewId()))
}

func TestHasFeatureFlag_SpecificOrg_Match(t *testing.T) {
	orgId := pure_utils.NewId()

	setFeatureFlagEnv(t, FEATURE_USER_SCORING, orgId.String())

	assert.True(t, HasFeatureFlag(FEATURE_USER_SCORING, orgId))
}

func TestHasFeatureFlag_SpecificOrg_NoMatch(t *testing.T) {
	setFeatureFlagEnv(t, FEATURE_USER_SCORING, pure_utils.NewId().String())

	assert.False(t, HasFeatureFlag(FEATURE_USER_SCORING, pure_utils.NewId()))
}

func TestHasFeatureFlag_MultipleOrgs(t *testing.T) {
	org1 := pure_utils.NewId()
	org2 := pure_utils.NewId()
	org3 := pure_utils.NewId()

	setFeatureFlagEnv(t, FEATURE_USER_SCORING, fmt.Sprintf("%s,%s", org1.String(), org2.String()))

	assert.True(t, HasFeatureFlag(FEATURE_USER_SCORING, org1))
	assert.True(t, HasFeatureFlag(FEATURE_USER_SCORING, org2))
	assert.False(t, HasFeatureFlag(FEATURE_USER_SCORING, org3))
}
