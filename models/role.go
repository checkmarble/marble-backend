package models

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RbacRole struct {
	Id          uuid.UUID
	OrgId       uuid.UUID
	Slug        string
	Name        string
	Permissions []Permission
}

type RoleBindingConditions struct {
	NotBefore *time.Time `json:"notBefore,omitempty"`
	NotAfter  *time.Time `json:"notAfter,omitempty"`
}

func (conditions RoleBindingConditions) Validate() error {
	if conditions.NotBefore != nil && conditions.NotAfter != nil && !conditions.NotBefore.Before(*conditions.NotAfter) {
		return fmt.Errorf("notBefore must be before notAfter: %w", BadParameterError)
	}

	return nil
}

type RoleBinding struct {
	Id           uuid.UUID
	OrgId        uuid.UUID
	UserId       *UserId
	ApiKeyId     *uuid.UUID
	Role         Role
	CustomRoleId *uuid.UUID
	Conditions   RoleBindingConditions
	Permissions  []Permission
}

func (b RoleBinding) IsActive(now time.Time) bool {
	if b.Conditions.NotBefore != nil && now.Before(*b.Conditions.NotBefore) {
		return false
	}

	return b.Conditions.NotAfter == nil || now.Before(*b.Conditions.NotAfter)
}

func (b RoleBinding) RoleName() Role {
	return b.Role
}

func NewNativeRoleBinding(role Role) RoleBinding {
	return RoleBinding{Role: role, Permissions: role.Permissions()}
}

func RoleNames(bindings []RoleBinding) []Role {
	roles := make([]Role, 0, len(bindings))

	for _, binding := range bindings {
		if role := binding.RoleName(); role != "" && !slices.Contains(roles, role) {
			roles = append(roles, role)
		}
	}

	return roles
}

func NativeRoleBindings(roles []Role) []RoleBinding {
	bindings := make([]RoleBinding, 0, len(roles))

	for _, role := range roles {
		bindings = append(bindings, NewNativeRoleBinding(role))
	}

	return bindings
}

// LegacyRoleValue returns the value stored in the singular role column for
// rollback compatibility. The legacy schema can represent only one native
// role, so the first native binding is used.
func LegacyRoleValue(bindings []RoleBinding) int {
	values := map[Role]int{
		SYSTEM:       0,
		VIEWER:       1,
		BUILDER:      2,
		PUBLISHER:    3,
		ADMIN:        4,
		API_CLIENT:   5,
		MARBLE_ADMIN: 6,
		ANALYST:      9,
	}

	for _, binding := range bindings {
		if value, ok := values[binding.Role]; ok {
			return value
		}
	}

	return 0
}

type Role string

var customRolePattern = regexp.MustCompile(`^org/[a-zA-Z\.]+$`)

func (r Role) IsCustom() bool {
	return strings.HasPrefix(string(r), "org/")
}

func (r Role) IsValidCustom() bool {
	return customRolePattern.MatchString(string(r))
}

// Do not remove or reorder entries here, even if a role if deleted, since the
// value is used for identity.
const (
	SYSTEM       Role = "SYSTEM"
	VIEWER       Role = "VIEWER"
	BUILDER      Role = "BUILDER"
	PUBLISHER    Role = "PUBLISHER"
	ADMIN        Role = "ADMIN"
	API_CLIENT   Role = "API_CLIENT"
	MARBLE_ADMIN Role = "MARBLE_ADMIN"
	ANALYST      Role = "ANALYST"
)

func GetValidUserRoles() []Role {
	return []Role{
		VIEWER,
		BUILDER,
		PUBLISHER,
		ADMIN,
		MARBLE_ADMIN,
		ANALYST,
	}
}

func (r Role) Permissions() []Permission {
	permissions := ROLES_PERMISSIONS[r]
	if permissions == nil {
		return []Permission{}
	}
	return permissions
}

func (r Role) HasPermission(permission Permission) bool {
	return slices.Contains(r.Permissions(), permission)
}
