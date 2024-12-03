package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

// type CreateScenarioTestRunBody struct {
// 	ScenarioIterationId string `json:"scenario_iteration_id"`
// 	ScenarioId          string `json:"scenario_id"`
// 	RefIterationId      string `json:"ref_iteration_id"`
// 	PhantomIterationId  string `json:"phantom_iteration_id"`
// 	StartDate           string `json:"start_date"`
// 	EndDate             string `json:"end_date"`
// 	Period              string `json:"period"`
// }

type CreateScenarioTestRunBody struct {
	TestIterationId string `json:"test_iteration_id"`
	ScenarioId      string `json:"scenario_id"`
	EndDate         string `json:"end_date"`
}

type ScenarioTestRunResp struct {
	Id              string `json:"id"`
	ScenarioId      string `json:"scenario_id"`
	RefIterationId  string `json:"ref_iteration_id"`
	TestIterationId string `json:"test_iteration_id"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	CreatorId       string `json:"creator_id"`
	Status          string `json:"status"`
}

func AdaptScenarioTestRunDto(s models.ScenarioTestRun) ScenarioTestRunResp {
	return ScenarioTestRunResp{
		Id:              s.Id,
		StartDate:       s.CreatedAt.String(),
		EndDate:         s.ExpiresAt.String(),
		Status:          s.Status.String(),
		RefIterationId:  s.ScenarioLiveIterationId,
		ScenarioId:      s.ScenarioId,
		TestIterationId: s.ScenarioIterationId,
	}
}

func AdaptCreateScenarioTestRunBody(dto CreateScenarioTestRunBody) (models.ScenarioTestRunInput, error) {
	layout := "2006-01-02T15:04:05"
	ed, err := time.Parse(layout, dto.EndDate)
	if err != nil {
		return models.ScenarioTestRunInput{}, err
	}
	return models.ScenarioTestRunInput{
		ScenarioId:         dto.ScenarioId,
		EndDate:            ed,
		PhantomIterationId: dto.TestIterationId,
	}, nil
}
