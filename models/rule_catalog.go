package models

type RuleCatalog struct {
	Rules []RuleCatalogRule `json:"rules"`
}

type RuleCatalogRule struct {
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	Prompt      string            `json:"prompt"`
}
