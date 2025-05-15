package v1

import (
	"testing"

	"github.com/gavv/httpexpect/v2"
)

func PublicApiV1(t *testing.T, e *httpexpect.Expect) {
	sanctionChecks(t, e)
	whitelists(t, e)
	decisions(t, e)
}
