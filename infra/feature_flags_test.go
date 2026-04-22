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
	t.Setenv(fmt.Sprintf("ENABLE_%s", string(TEST_UNUSED_FEATURE_FLAG)), "")

	assert.False(t, HasGlobalFeatureFlag(TEST_UNUSED_FEATURE_FLAG))
}

func TestHasGlobalFeatureFlag_Set(t *testing.T) {
	setFeatureFlagEnv(t, TEST_UNUSED_FEATURE_FLAG, "all")

	assert.True(t, HasGlobalFeatureFlag(TEST_UNUSED_FEATURE_FLAG))
}

func TestHasFeatureFlag_EnvNotSet(t *testing.T) {
	t.Setenv(fmt.Sprintf("ENABLE_%s", string(TEST_UNUSED_FEATURE_FLAG)), "")

	orgId := pure_utils.NewId()

	assert.False(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, orgId))
}

func TestHasFeatureFlag_All(t *testing.T) {
	setFeatureFlagEnv(t, TEST_UNUSED_FEATURE_FLAG, "all")

	assert.True(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, pure_utils.NewId()))
}

func TestHasFeatureFlag_SpecificOrg_Match(t *testing.T) {
	orgId := pure_utils.NewId()

	setFeatureFlagEnv(t, TEST_UNUSED_FEATURE_FLAG, orgId.String())

	assert.True(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, orgId))
}

func TestHasFeatureFlag_SpecificOrg_NoMatch(t *testing.T) {
	setFeatureFlagEnv(t, TEST_UNUSED_FEATURE_FLAG, pure_utils.NewId().String())

	assert.False(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, pure_utils.NewId()))
}

func TestHasFeatureFlag_MultipleOrgs(t *testing.T) {
	org1 := pure_utils.NewId()
	org2 := pure_utils.NewId()
	org3 := pure_utils.NewId()

	setFeatureFlagEnv(t, TEST_UNUSED_FEATURE_FLAG, fmt.Sprintf("%s,%s", org1.String(), org2.String()))

	assert.True(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, org1))
	assert.True(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, org2))
	assert.False(t, HasFeatureFlag(TEST_UNUSED_FEATURE_FLAG, org3))
}
