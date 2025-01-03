package models

import "time"

type OrganizationEntitlement struct {
	Id             string
	OrganizationId string
	FeatureId      string
	Availability   FeatureAvailability
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateOrganizationEntitlementInput struct {
	Id             string
	OrganizationId string
	FeatureId      string
	Availability   FeatureAvailability
}

type UpdateOrganizationEntitlementInput struct {
	Id             string
	OrganizationId string
	FeatureId      string
	Availability   FeatureAvailability
}
