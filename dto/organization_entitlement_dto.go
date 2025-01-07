package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIOrganizationEntitlement struct {
	Id             string    `json:"id"`
	OrganizationId string    `json:"organization_id"`
	FeatureId      string    `json:"feature_id"`
	Availability   string    `json:"availability"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func AdaptOrganizationEntitlementDto(f models.OrganizationEntitlement) APIOrganizationEntitlement {
	return APIOrganizationEntitlement{
		Id:             f.Id,
		OrganizationId: f.OrganizationId,
		FeatureId:      f.FeatureId,
		Availability:   f.Availability.String(),
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
	}
}

type UpdateOrganizationEntitlementBodyDto struct {
	FeatureId    string `json:"feature_id" binding:"required"`
	Availability string `json:"availability" binding:"required"`
}
