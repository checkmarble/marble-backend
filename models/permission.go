package models

type Permission int

const (
	DECISION_READ Permission = iota
	DECISION_CREATE
	INGESTION
	SCENARIO_READ
	SCENARIO_CREATE
	SCENARIO_PUBLISH
	DATA_MODEL_READ
	DATA_MODEL_WRITE
	APIKEY_READ
	ORGANIZATIONS_LIST
	ORGANIZATIONS_CREATE
	ORGANIZATIONS_DELETE
	USER_CREATE
	MARBLE_USER_CREATE
	MARBLE_USER_DELETE
	ANY_ORGANIZATION_ID_IN_CONTEXT
	CUSTOM_LISTS_READ
	CUSTOM_LISTS_CREATE
	MARBLE_USER_LIST
)

func (r Permission) String() string {
	return [...]string{
		"DECISION_READ",
		"DECISION_CREATE",
		"INGESTION",
		"SCENARIO_READ",
		"SCENARIO_CREATE",
		"SCENARIO_PUBLISH",
		"DATA_MODEL_READ",
		"DATA_MODEL_WRITE",
		"APIKEY_READ",
		"ORGANIZATIONS_LIST",
		"ORGANIZATIONS_CREATE",
		"ORGANIZATIONS_DELETE",
		"USER_CREATE",
		"MARBLE_USER_CREATE",
		"MARBLE_USER_DELETE",
		"ANY_ORGANIZATION_ID_IN_CONTEXT",
		"CUSTOM_LISTS_READ",
		"CUSTOM_LISTS_PUBLISH",
		"MARBLE_USER_LIST",
	}[r]
}
