package httpmodels

type HTTPLexisNexisCatalogTopicsResponse struct {
	Topics []string `json:"properties.topics"` //nolint:tagliatelle
}
