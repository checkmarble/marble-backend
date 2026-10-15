package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const (
	TABLE_GROUPS            = "groups"
	TABLE_GROUP_MEMBERSHIPS = "group_memberships"
)

type DbGroup struct {
	Id             uuid.UUID `db:"id"`
	OrganizationId uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
}

var SelectGroupColumns = utils.ColumnList[DbGroup]()

func AdaptGroup(db DbGroup) (models.Group, error) {
	return models.Group{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		Name:           db.Name,
	}, nil
}

type DbGroupMembership struct {
	GroupId uuid.UUID `db:"group_id"`
	UserId  uuid.UUID `db:"user_id"`
}

var SelectGroupMembershipColumns = utils.ColumnList[DbGroupMembership]()

func AdaptGroupMembership(db DbGroupMembership) (models.GroupMembership, error) {
	return models.GroupMembership{
		GroupId: db.GroupId,
		UserId:  models.UserId(db.UserId.String()),
	}, nil
}

// SelectUserGroupIds selects, as grant principal IDs, the groups whose grants
// apply to a user: the groups it is a member of, in organizations of its own
// tenant. Its only placeholder is the user ID.
const SelectUserGroupIds = `SELECT gr.id::text
	FROM ` + TABLE_GROUP_MEMBERSHIPS + ` gm
	JOIN ` + TABLE_GROUPS + ` gr ON gr.id = gm.group_id AND gr.deleted_at IS NULL
	JOIN organizations o ON o.id = gr.organization_id AND o.deleted_at IS NULL
	JOIN users u ON u.id = gm.user_id AND u.deleted_at IS NULL
	JOIN organizations home ON home.id = u.organization_id AND home.tenant_id = o.tenant_id
	WHERE gm.user_id = ?`
