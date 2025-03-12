package models

var VIEWER_PERMISSIONS = []Permission{
	ANALYTICS_READ,
	DECISION_READ,
	SCENARIO_READ,
	DATA_MODEL_READ,
	CUSTOM_LISTS_READ,
	CASE_READ_WRITE,
	WEBHOOK_EVENT,
	READ_SNOOZES,
	CREATE_SNOOZE,
	TAG_READ,
	MARBLE_USER_READ,
	MARBLE_USER_LIST,
	MARBLE_USER_UPDATE,
}

var (
	BUILDER_PERMISSIONS   = append(VIEWER_PERMISSIONS, SCENARIO_CREATE)
	PUBLISHER_PERMISSIONS = append(BUILDER_PERMISSIONS, SCENARIO_PUBLISH, CUSTOM_LISTS_EDIT)
	ADMIN_PERMISSIONS     = append(
		PUBLISHER_PERMISSIONS,
		APIKEY_READ,
		APIKEY_CREATE,
		DATA_MODEL_WRITE,
		DECISION_CREATE,
		PHANTOM_DECISION_CREATE,
		INGESTION,
		MARBLE_USER_CREATE,
		MARBLE_USER_DELETE,
		INBOX_EDITOR,
		WEBHOOK,
		TAG_CREATE,
		TAG_UPDATE,
		TAG_DELETE,
		ORGANIZATIONS_UPDATE,
	)
)

var ROLES_PERMISSIOMS = map[Role][]Permission{
	NO_ROLE:   {},
	VIEWER:    VIEWER_PERMISSIONS,
	BUILDER:   BUILDER_PERMISSIONS,
	PUBLISHER: PUBLISHER_PERMISSIONS,
	ADMIN:     ADMIN_PERMISSIONS,
	API_CLIENT: {
		SCENARIO_READ,
		SCENARIO_CREATE,
		DECISION_READ,
		DECISION_CREATE,
		PHANTOM_DECISION_CREATE,
		INGESTION,
		DATA_MODEL_READ,
		CUSTOM_LISTS_READ,
		CUSTOM_LISTS_EDIT,
		WEBHOOK_EVENT,
	},
	TRANSFER_CHECK_API_CLIENT: {
		TRANSFER_READ,
		TRANSFER_CREATE,
		TRANSFER_UPDATE,
		DECISION_CREATE,
		PHANTOM_DECISION_CREATE,
	},
	TRANSFER_CHECK_USER: {
		TRANSFER_READ,
		TRANSFER_UPDATE,
		PARTNER_READ,
		TRANSFER_ALERT_READ,
		TRANSFER_ALERT_UPDATE,
		TRANSFER_ALERT_CREATE,
	},
	MARBLE_ADMIN: append(
		ADMIN_PERMISSIONS,
		ORGANIZATIONS_LIST,
		ORGANIZATIONS_CREATE,
		ORGANIZATIONS_UPDATE,
		ORGANIZATIONS_DELETE,
		ANY_ORGANIZATION_ID_IN_CONTEXT,
		ANY_PARTNER_ID_IN_CONTEXT,
		PARTNER_LIST,
		PARTNER_READ,
		PARTNER_CREATE,
		PARTNER_UPDATE,
		LICENSE_LIST,
		LICENSE_CREATE,
		LICENSE_UPDATE,
	),
}
