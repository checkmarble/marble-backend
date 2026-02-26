package infra

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type featureFlag string

const (
	FEATURE_USER_SCORING featureFlag = "USER_SCORING"
)

func HasGlobalFeatureFlag(flag featureFlag) bool {
	return getFeatureFlagEnv(flag) != ""
}

func HasFeatureFlag(flag featureFlag, orgId uuid.UUID) bool {
	env := getFeatureFlagEnv(flag)

	if env == "" {
		return false
	}

	for org := range strings.SplitSeq(env, ",") {
		if org == "all" {
			return true
		}
		if org == orgId.String() {
			return true
		}
	}

	return false
}

func RouteWithFeatureFlag(parent gin.IRoutes, flag featureFlag, cb func(sub gin.IRoutes)) {
	if !HasGlobalFeatureFlag(flag) {
		return
	}

	sub := parent.Use(featureFlagMiddleware(flag))

	cb(sub)
}

func featureFlagMiddleware(flag featureFlag) func(*gin.Context) {
	return func(c *gin.Context) {
		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if err != nil {
			c.Next()
			return
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

func getFeatureFlagEnv(flag featureFlag) string {
	return os.Getenv(fmt.Sprintf("ENABLE_%s", string(flag)))
}
