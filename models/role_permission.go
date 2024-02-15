package models

var VIEWER_PERMISSIONS = []Permission{
	DECISION_READ,
	SCENARIO_READ,
	DATA_MODEL_READ,
	CUSTOM_LISTS_READ,
	CASE_READ_WRITE,
}

var (
	BUILDER_PERMISSIONS   = append(VIEWER_PERMISSIONS, SCENARIO_CREATE)
	PUBLISHER_PERMISSIONS = append(BUILDER_PERMISSIONS, SCENARIO_PUBLISH, CUSTOM_LISTS_CREATE)
	ADMIN_PERMISSIONS     = append(
		PUBLISHER_PERMISSIONS,
		APIKEY_READ,
		APIKEY_CREATE,
		ANALYTICS_READ,
		DATA_MODEL_WRITE,
		DECISION_CREATE,
		INGESTION,
		SCENARIO_CREATE,
		SCENARIO_PUBLISH,
		MARBLE_USER_CREATE,
		MARBLE_USER_DELETE,
		INBOX_EDITOR,
	)
)

var ROLES_PERMISSIOMS = map[Role][]Permission{
	NO_ROLE:    {},
	VIEWER:     VIEWER_PERMISSIONS,
	BUILDER:    BUILDER_PERMISSIONS,
	PUBLISHER:  PUBLISHER_PERMISSIONS,
	ADMIN:      ADMIN_PERMISSIONS,
	API_CLIENT: {SCENARIO_READ, SCENARIO_CREATE, DECISION_READ, DECISION_CREATE, INGESTION, DATA_MODEL_READ, CUSTOM_LISTS_READ, CUSTOM_LISTS_CREATE},
	MARBLE_ADMIN: append(
		ADMIN_PERMISSIONS,
		ORGANIZATIONS_LIST,
		ORGANIZATIONS_CREATE,
		ORGANIZATIONS_DELETE,
		ANY_ORGANIZATION_ID_IN_CONTEXT,
		CUSTOM_LISTS_READ,
		CUSTOM_LISTS_CREATE,
		DATA_MODEL_WRITE,
		SCENARIO_CREATE,
		SCENARIO_PUBLISH,
		MARBLE_USER_LIST,
	),
}
