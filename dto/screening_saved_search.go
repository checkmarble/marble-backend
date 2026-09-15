package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type ScreeningSavedSearch struct {
	Id        uuid.UUID            `json:"id"`
	OrgId     uuid.UUID            `json:"org_id"`
	Name      string               `json:"name"`
	Provider  string               `json:"provider"`
	Config    ScreeningFreeformDto `json:"config"`
	CreatedAt time.Time            `json:"created_at"`
	DeletedAt *time.Time           `json:"deleted_at,omitempty"`
}

func AdaptScreeningSavedSearch(m models.ScreeningSavedSearch) (ScreeningSavedSearch, error) {
	var cfg ScreeningFreeformDto

	if err := json.Unmarshal(m.Config, &cfg); err != nil {
		return ScreeningSavedSearch{}, err
	}

	return ScreeningSavedSearch{
		Id:        m.Id,
		OrgId:     m.OrgId,
		Name:      m.Name,
		Provider:  m.Provider,
		Config:    cfg,
		CreatedAt: m.CreatedAt,
		DeletedAt: m.DeletedAt,
	}, nil
}

type CreateScreeningSavedSearch struct {
	Name   string               `json:"name" binding:"required,gt=0"`
	Config ScreeningFreeformDto `json:"config"`
}
