package models

// OrganizationUserGrant is a user of a tenant, with its role bindings in one
// organization of the tenant, which may not be its home organization.
type OrganizationUserGrant struct {
	User         User
	RoleBindings []RoleBinding
}

type OrganizationMembership struct {
	Organization Organization
	Roles        []Role
}
