package api

type contextKey int

const (
	contextKeyOrgID contextKey = iota
	contextKeyClaims
	contextKeyTokenType
	contextKeyTokenRole
)
