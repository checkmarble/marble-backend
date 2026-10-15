package models

import "github.com/google/uuid"

// Group gives its role bindings, in its organization, to its members.
type Group struct {
	Id             uuid.UUID
	OrganizationId uuid.UUID
	Name           string
	RoleBindings   []RoleBinding
	MemberIds      []UserId
}

type GroupMembership struct {
	GroupId uuid.UUID
	UserId  UserId
}
