package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// test run response DTO
type ScenarioTestRunResp struct {
	Id              string    `json:"id"`
	ScenarioId      string    `json:"scenario_id"`
	RefIterationId  string    `json:"ref_iteration_id"`
	TestIterationId string    `json:"test_iteration_id"`
	StartDate       time.Time `json:"start_date"`
	EndDate         time.Time `json:"end_date"`
	CreatorId       string    `json:"creator_id"`
	Status          string    `json:"status"`
}

func AdaptScenarioTestRunDto(s models.ScenarioTestRun) ScenarioTestRunResp {
	return ScenarioTestRunResp{
		Id:              s.Id,
		StartDate:       s.CreatedAt,
		EndDate:         s.ExpiresAt,
		Status:          s.Status.String(),
		RefIterationId:  s.ScenarioLiveIterationId,
		ScenarioId:      s.ScenarioId,
		TestIterationId: s.ScenarioIterationId,
	}
}

// test run create input DTO
type CreateScenarioTestRunBody struct {
	TestIterationId string    `json:"test_iteration_id"`
	ScenarioId      string    `json:"scenario_id"`
	EndDate         time.Time `json:"end_date"`
}

func AdaptCreateScenarioTestRunBody(dto CreateScenarioTestRunBody) (models.ScenarioTestRunInput, error) {
	return models.ScenarioTestRunInput{
		ScenarioId:         dto.ScenarioId,
		EndDate:            dto.EndDate,
		PhantomIterationId: dto.TestIterationId,
	}, nil
}

// rule execution stats DTO. Contains statistics on rule executions for either the live version or the tested version.
type RuleExecutionData struct {
	Version      string `json:"version"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	StableRuleId string `json:"stable_rule_id"`
	Total        int    `json:"total"`
}

func ProcessRuleExecutionDataDtoFromModels(inputs []models.RuleExecutionStat) []RuleExecutionData {
	result := make([]RuleExecutionData, len(inputs))
	for i, input := range inputs {
		item := RuleExecutionData{
			Version:      input.Version,
			Name:         input.Name,
			Status:       input.Outcome,
			Total:        input.Total,
			StableRuleId: input.StableRuleId,
		}
		result[i] = item
	}
	return result
}

// Decision stats DTO. Contains statistics on decisions created by the live version or the tested version.
type DecisionData struct {
	Version string `json:"version"`
	Outcome string `json:"outcome"`
	Total   int    `json:"total"`
}

func ProcessDecisionDataDtoFromModels(inputs []models.DecisionsByVersionByOutcome) []DecisionData {
	result := make([]DecisionData, len(inputs))
	for i, input := range inputs {
		item := DecisionData{
			Version: input.Version,
			Outcome: input.Outcome,
			Total:   input.Count,
		}
		result[i] = item
	}
	return result
}
