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

type ScenarionResp struct {
	ScenarioId string        `json:"scenario_id"`
	Period     time.Duration `json:"period"`
}

func AdaptScenarioTestRunDto(s models.ScenarioTestRun) ScenarionResp {
	return ScenarionResp{
		ScenarioId: s.ScenarioId,
		Period:     s.Period,
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
