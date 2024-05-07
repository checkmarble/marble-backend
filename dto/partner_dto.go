package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type Partner struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
}

func AdaptPartnerDto(partner models.Partner) Partner {
	return Partner{
		Id:        partner.Id,
		CreatedAt: partner.CreatedAt,
		Name:      partner.Name,
	}
}

type PartnerCreateBody struct {
	Name string `json:"name"`
}

func AdaptPartnerCreateInput(dto PartnerCreateBody) models.PartnerCreateInput {
	return models.PartnerCreateInput{
		Name: dto.Name,
	}
}

type PartnerUpdateBody struct {
	Name string `json:"name"`
}

func AdaptPartnerUpdateInput(dto PartnerUpdateBody) models.PartnerUpdateInput {
	return models.PartnerUpdateInput{
		Name: dto.Name,
	}
}
