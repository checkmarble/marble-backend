package models

var VIEWER_PERMISSIONS = []Permission{
	DECISION_READ,
	SCENARIO_READ,
	DATA_MODEL_READ,
}

var BUILDER_PERMISSIONS = append(VIEWER_PERMISSIONS, SCENARIO_CREATE)
var PUBLISHER_PERMISSIONS = append(BUILDER_PERMISSIONS, SCENARIO_PUBLISH)
var ADMIN_PERMISSIONS = append(PUBLISHER_PERMISSIONS, USER_CREATE)

var ROLES_PERMISSIOMS = map[Role][]Permission{
	NO_ROLE:      {},
	VIEWER:       VIEWER_PERMISSIONS,
	BUILDER:      BUILDER_PERMISSIONS,
	PUBLISHER:    PUBLISHER_PERMISSIONS,
	ADMIN:        ADMIN_PERMISSIONS,
	API_CLIENT:   {SCENARIO_READ, DECISION_READ, DECISION_CREATE, INGESTION, DATA_MODEL_READ},
	MARBLE_ADMIN: append(ADMIN_PERMISSIONS, ORGANIZATIONS_LIST, ORGANIZATIONS_CREATE, ANY_ORGANIZATION_ID_IN_CONTEXT),
}
