package api

type contextKey int

const (
	contextKeyOrgID contextKey = iota
	contextKeyTokenType
	contextKeyTokenRole
)
