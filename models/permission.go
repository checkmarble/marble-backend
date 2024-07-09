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
	APIKEY_CREATE
	ANALYTICS_READ
	ORGANIZATIONS_LIST
	ORGANIZATIONS_CREATE
	ORGANIZATIONS_DELETE
	USER_CREATE
	MARBLE_USER_CREATE
	MARBLE_USER_DELETE
	ANY_ORGANIZATION_ID_IN_CONTEXT
	ANY_PARTNER_ID_IN_CONTEXT
	CUSTOM_LISTS_READ
	CUSTOM_LISTS_CREATE
	MARBLE_USER_LIST
	CASE_READ_WRITE
	INBOX_EDITOR
	TRANSFER_READ
	TRANSFER_UPDATE
	TRANSFER_CREATE
	TRANSFER_ALERT_READ
	TRANSFER_ALERT_UPDATE
	TRANSFER_ALERT_CREATE
	PARTNER_LIST
	PARTNER_CREATE
	PARTNER_READ
	PARTNER_UPDATE
	LICENSE_LIST
	LICENSE_CREATE
	LICENSE_UPDATE
	WEBHOOK_CREATE
	WEBHOOK_SEND
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
		"APIKEY_CREATE",
		"ANALYTICS_READ",
		"ORGANIZATIONS_LIST",
		"ORGANIZATIONS_CREATE",
		"ORGANIZATIONS_DELETE",
		"USER_CREATE",
		"MARBLE_USER_CREATE",
		"MARBLE_USER_DELETE",
		"ANY_ORGANIZATION_ID_IN_CONTEXT",
		"ANY_PARTNER_ID_IN_CONTEXT",
		"CUSTOM_LISTS_READ",
		"CUSTOM_LISTS_PUBLISH",
		"MARBLE_USER_LIST",
		"CASE_READ_WRITE",
		"INBOX_EDITOR",
		"TRANSFER_READ",
		"TRANSFER_UPDATE",
		"TRANSFER_CREATE",
		"TRANSFER_ALERT_READ",
		"TRANSFER_ALERT_UPDATE",
		"TRANSFER_ALERT_CREATE",
		"PARTNER_LIST",
		"PARTNER_CREATE",
		"PARTNER_READ",
		"PARTNER_UPDATE",
		"LICENSE_LIST",
		"LICENSE_CREATE",
		"LICENSE_UPDATE",
		"WEBHOOK_CREATE",
		"WEBHOOK_SEND",
	}[r]
}
