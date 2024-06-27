package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type Partner struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Bic       string    `json:"bic"`
}

func AdaptPartnerDto(partner models.Partner) Partner {
	return Partner{
		Id:        partner.Id,
		CreatedAt: partner.CreatedAt,
		Name:      partner.Name,
		Bic:       partner.Bic,
	}
}

type PartnerCreateBody struct {
	Name string `json:"name"`
	Bic  string `json:"bic"`
}

func AdaptPartnerCreateInput(dto PartnerCreateBody) models.PartnerCreateInput {
	return models.PartnerCreateInput{
		Name: dto.Name,
		Bic:  dto.Bic,
	}
}

type PartnerUpdateBody struct {
	Name null.String `json:"name"`
	Bic  null.String `json:"bic"`
}

func AdaptPartnerUpdateInput(dto PartnerUpdateBody) models.PartnerUpdateInput {
	return models.PartnerUpdateInput{
		Name: dto.Name,
		Bic:  dto.Bic,
	}
}
