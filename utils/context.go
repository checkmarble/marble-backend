package utils

import (
	"context"
	"fmt"
)

type ContextKey int

const (
	ContextKeyOrgID ContextKey = iota
	ContextKeyClaims
	ContextKeyTokenType
	ContextKeyTokenRole
)

var ErrOrgNotInContext = fmt.Errorf("organization ID not found in request context")

func OrgIDFromCtx(ctx context.Context) (id string, err error) {

	orgID, found := ctx.Value(ContextKeyOrgID).(string)

	if !found {
		return "", ErrOrgNotInContext
	}

	return orgID, nil
}
