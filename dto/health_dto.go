package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type HealthStatusResponse struct {
	Status  bool                       `json:"status"`
	Details []HealthItemStatusResponse `json:"details"`
}

type HealthItemStatusResponse struct {
	Name   string `json:"name"`
	Status bool   `json:"status"`
}

func AdaptHealthItemStatus(status models.HealthItemStatus) HealthItemStatusResponse {
	return HealthItemStatusResponse{
		Name:   string(status.Name),
		Status: status.Status,
	}
}

func AdaptHealthStatus(status models.HealthStatus) HealthStatusResponse {
	return HealthStatusResponse{
		Status:  status.IsHealthy(),
		Details: pure_utils.Map(status.Statuses, AdaptHealthItemStatus),
	}
}
