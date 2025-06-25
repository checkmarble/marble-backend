package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type LivenessStatusResponse struct {
	Status []LivenessItemStatusResponse `json:"status"`
}

type LivenessItemStatusResponse struct {
	Name   string `json:"name"`
	IsLive bool   `json:"is_live"`
}

func AdaptLivenessItemStatus(status models.LivenessItemStatus) LivenessItemStatusResponse {
	return LivenessItemStatusResponse{
		Name:   string(status.Name),
		IsLive: status.IsLive,
	}
}

func AdaptLivenessStatus(status models.LivenessStatus) LivenessStatusResponse {
	return LivenessStatusResponse{
		Status: pure_utils.Map(status.Statuses, AdaptLivenessItemStatus),
	}
}
