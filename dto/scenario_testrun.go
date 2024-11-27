package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type CreateScenarioTestRunBody struct {
	ScenarioIterationId string `json:"scenario_iteration_id"`
	ScenarioId          string `json:"scenario_id"`
	Period              string `json:"period"`
}

type ScenarioTestRunResp struct {
	ScenarioId string    `json:"scenario_id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Status     string    `json:"status"`
}

func AdaptScenarioTestRunDto(s models.ScenarioTestRun) ScenarioTestRunResp {
	return ScenarioTestRunResp{
		ScenarioId: s.ScenarioId,
		CreatedAt:  s.CreatedAt,
		ExpiresAt:  s.Expires,
		Status:     s.Status.String(),
	}
}

func AdaptCreateScenarioTestRunBody(dto CreateScenarioTestRunBody) (models.ScenarioTestRunInput, error) {
	p, err := time.ParseDuration(dto.Period)
	if err != nil {
		return models.ScenarioTestRunInput{}, err
	}
	return models.ScenarioTestRunInput{
		ScenarioIterationId: dto.ScenarioIterationId,
		ScenarioId:          dto.ScenarioId,
		Period:              p,
	}, nil
}
