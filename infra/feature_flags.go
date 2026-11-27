package infra

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type featureFlag string

const (
	TEST_UNUSED_FEATURE_FLAG            featureFlag = "TEST_UNUSED_FEATURE_FLAG"
	ASYNC_DECISION_STORAGE_FEATURE_FLAG featureFlag = "ASYNC_DECISION_STORAGE"
)

var knownFeatureFlags = []featureFlag{
	ASYNC_DECISION_STORAGE_FEATURE_FLAG,
}

var (
	databaseFeatureFlagsLock sync.RWMutex
	databaseFeatureFlags     = map[featureFlag]string{}
)

func IsKnownFeatureFlag(flag string) bool {
	return slices.Contains(knownFeatureFlags, featureFlag(flag))
}

func KnownFeatureFlags() []string {
	flags := make([]string, len(knownFeatureFlags))

	for idx, flag := range knownFeatureFlags {
		flags[idx] = string(flag)
	}

	return flags
}

func SetFeatureFlagsFromDatabase(flags map[string]string) {
	values := make(map[featureFlag]string, len(flags))

	for flag, value := range flags {
		values[featureFlag(flag)] = value
	}

	databaseFeatureFlagsLock.Lock()
	defer databaseFeatureFlagsLock.Unlock()

	databaseFeatureFlags = values
}

func HasGlobalFeatureFlag(flag featureFlag) bool {
	return getFeatureFlagValue(flag) != ""
}

func HasFeatureFlag(flag featureFlag, orgId uuid.UUID) bool {
	value := getFeatureFlagValue(flag)

	if value == "" {
		return false
	}

	for org := range strings.SplitSeq(value, ",") {
		if org == "all" {
			return true
		}
		if org == orgId.String() {
			return true
		}
	}

	return false
}

func RouteWithFeatureFlag(parent gin.IRouter, flag featureFlag, cb func(sub gin.IRoutes)) {
	if !HasGlobalFeatureFlag(flag) {
		return
	}

	sub := parent.Group("/", featureFlagMiddleware(flag))

	cb(sub)
}

func featureFlagMiddleware(flag featureFlag) func(*gin.Context) {
	return func(c *gin.Context) {
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			orgId = uuid.Nil
		}

		switch HasFeatureFlag(flag, orgId) {
		case true:
			c.Next()
		case false:
			c.Status(http.StatusNotFound)
			c.Abort()
		}
	}
}

func getFeatureFlagValue(flag featureFlag) string {
	if value, ok := os.LookupEnv(fmt.Sprintf("ENABLE_%s", string(flag))); ok {
		return value
	}

	databaseFeatureFlagsLock.RLock()
	defer databaseFeatureFlagsLock.RUnlock()

	return databaseFeatureFlags[flag]
}
