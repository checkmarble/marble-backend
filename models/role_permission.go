package models

var VIEWER_PERMISSIONS = []Permission{
	DECISION_READ,
	SCENARIO_READ,
	DATA_MODEL_READ,
	CUSTOM_LISTS_READ,
}

var BUILDER_PERMISSIONS = append(VIEWER_PERMISSIONS, SCENARIO_CREATE)
var PUBLISHER_PERMISSIONS = append(BUILDER_PERMISSIONS, SCENARIO_PUBLISH, CUSTOM_LISTS_CREATE)
var ADMIN_PERMISSIONS = append(PUBLISHER_PERMISSIONS, USER_CREATE, SCENARIO_CREATE, SCENARIO_PUBLISH, APIKEY_READ, DATA_MODEL_WRITE, INGESTION)

var ROLES_PERMISSIOMS = map[Role][]Permission{
	NO_ROLE:    {},
	VIEWER:     VIEWER_PERMISSIONS,
	BUILDER:    BUILDER_PERMISSIONS,
	PUBLISHER:  PUBLISHER_PERMISSIONS,
	ADMIN:      ADMIN_PERMISSIONS,
	API_CLIENT: {SCENARIO_READ, SCENARIO_CREATE, DECISION_READ, DECISION_CREATE, INGESTION, DATA_MODEL_READ, CUSTOM_LISTS_READ, CUSTOM_LISTS_CREATE},
	MARBLE_ADMIN: append(
		ADMIN_PERMISSIONS,
		DECISION_CREATE,
		ORGANIZATIONS_LIST,
		ORGANIZATIONS_CREATE,
		ORGANIZATIONS_DELETE,
		ANY_ORGANIZATION_ID_IN_CONTEXT,
		MARBLE_USER_CREATE,
		MARBLE_USER_DELETE,
		CUSTOM_LISTS_READ,
		CUSTOM_LISTS_CREATE,
		DATA_MODEL_WRITE,
		SCENARIO_CREATE,
		SCENARIO_PUBLISH,
		MARBLE_USER_LIST,
	),
}
