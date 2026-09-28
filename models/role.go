package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
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
	NotBefore        *time.Time `json:"notBefore,omitempty"`        //nolint:tagliatelle
	NotAfter         *time.Time `json:"notAfter,omitempty"`         //nolint:tagliatelle
	UsedSecondFactor *bool      `json:"usedSecondFactor,omitempty"` //nolint:tagliatelle
	Networks         []Subnet   `json:"networks,omitempty"`
}

func (conditions RoleBindingConditions) Validate() error {
	if conditions.NotBefore != nil && conditions.NotAfter != nil && !conditions.NotBefore.Before(*conditions.NotAfter) {
		return fmt.Errorf("notBefore must be before notAfter: %w", BadParameterError)
	}

	return nil
}

type RoleBinding struct {
	Id           uuid.UUID
	TenantId     uuid.UUID
	OrgId        uuid.UUID
	UserId       *UserId
	ApiKeyId     *uuid.UUID
	Role         Role
	CustomRoleId *uuid.UUID
	Conditions   RoleBindingConditions
	Permissions  []Permission
}

// Clock is the time source caveats are evaluated against, satisfied by
// repositories/clock.Clock.
type Clock interface {
	Now() time.Time
}

// RoleBindingBundle holds what the caveats of role bindings are evaluated
// against.
type RoleBindingBundle struct {
	// Clock is the time source caveats are evaluated against. A nil clock
	// means the current time.
	Clock            Clock
	UsedSecondFactor bool
	// ClientIp is resolved on every request and is never part of the token.
	ClientIp net.IP
}

// Now returns the time caveats are evaluated at.
func (bundle RoleBindingBundle) Now() time.Time {
	if bundle.Clock == nil {
		return time.Now()
	}

	return bundle.Clock.Now()
}

func (b RoleBinding) IsActive(bundle RoleBindingBundle) bool {
	now := bundle.Now()

	if b.Conditions.NotBefore != nil && now.Before(*b.Conditions.NotBefore) {
		return false
	}
	if b.Conditions.NotAfter != nil && now.After(*b.Conditions.NotAfter) {
		return false
	}

	if b.Conditions.UsedSecondFactor != nil && *b.Conditions.UsedSecondFactor && !bundle.UsedSecondFactor {
		return false
	}

	// Like the organization allowed networks guard, an empty list is
	// unrestricted, and an unknown client IP fails open.
	if len(b.Conditions.Networks) > 0 && bundle.ClientIp != nil {
		inNetworks := slices.ContainsFunc(b.Conditions.Networks, func(subnet Subnet) bool {
			return subnet.Contains(bundle.ClientIp)
		})

		if !inNetworks {
			return false
		}
	}

	return true
}

// Equivalent reports whether two bindings grant the same role under the same
// conditions. Conditions are compared through their stored JSON form, which
// covers every caveat, after normalizing timestamps: the same instant may be
// read back from the database with a different offset than it was given.
func (b RoleBinding) Equivalent(other RoleBinding) bool {
	if b.Role != other.Role {
		return false
	}

	left, errLeft := json.Marshal(b.Conditions.normalized())
	right, errRight := json.Marshal(other.Conditions.normalized())

	return errLeft == nil && errRight == nil && bytes.Equal(left, right)
}

func (conditions RoleBindingConditions) normalized() RoleBindingConditions {
	if conditions.NotBefore != nil {
		notBefore := conditions.NotBefore.UTC()
		conditions.NotBefore = &notBefore
	}
	if conditions.NotAfter != nil {
		notAfter := conditions.NotAfter.UTC()
		conditions.NotAfter = &notAfter
	}

	return conditions
}

// AppliesTo reports whether the binding grants its role when acting within
// the given organization (and its tenant). Without an organization, only
// platform-scoped bindings apply.
func (b RoleBinding) AppliesTo(orgId, tenantId uuid.UUID) bool {
	if orgId == uuid.Nil {
		return b.OrgId == uuid.Nil && b.TenantId == uuid.Nil
	}

	return b.OrgId == orgId || (b.TenantId != uuid.Nil && b.TenantId == tenantId)
}

func ScopeRoleBindings(bindings []RoleBinding, orgId, tenantId uuid.UUID) []RoleBinding {
	scoped := make([]RoleBinding, 0, len(bindings))

	for _, binding := range bindings {
		if binding.AppliesTo(orgId, tenantId) {
			scoped = append(scoped, binding)
		}
	}

	return scoped
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

var CUSTOM_ROLE_PATTERN = regexp.MustCompile(`^org/[a-zA-Z\.]+$`)

func (r Role) IsCustom() bool {
	return strings.HasPrefix(string(r), "org/")
}

func (r Role) IsValidCustom() bool {
	return CUSTOM_ROLE_PATTERN.MatchString(string(r))
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
	TENANT_ADMIN Role = "TENANT_ADMIN"
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

func GetValidOrganizationGrantRoles() []Role {
	return []Role{
		VIEWER,
		BUILDER,
		PUBLISHER,
		ADMIN,
		ANALYST,
	}
}

func (r Role) String() string {
	return string(r)
}

// RoleFromString parses a native role name, returning an empty role for
// unknown or internal (SYSTEM) roles.
func RoleFromString(s string) Role {
	switch role := Role(s); role {
	case VIEWER, BUILDER, PUBLISHER, ADMIN, API_CLIENT, MARBLE_ADMIN, ANALYST, TENANT_ADMIN:
		return role
	}
	return ""
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
