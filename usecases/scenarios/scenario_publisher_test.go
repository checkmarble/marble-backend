package scenarios

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func TestScenarioPublisher_PublishOrUnpublishIteration_unpublish_nominal(t *testing.T) {
	Iteration := models.ScenarioIteration{
		Id: uuid.New().String(),
	}

	scenario := models.Scenario{
		OrganizationId: uuid.New().String(),
		Id:             uuid.New().String(),
		LiveVersionID:  utils.Ptr(Iteration.Id),
	}

	scenarioAndIteration := ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: Iteration,
	}

	createScenarioInput := models.CreateScenarioPublicationInput{
		OrganizationId:      scenarioAndIteration.Scenario.OrganizationId,
		ScenarioId:          scenarioAndIteration.Scenario.Id,
		ScenarioIterationId: *scenarioAndIteration.Scenario.LiveVersionID,
		PublicationAction:   models.Unpublish,
	}

	scenarioPublication := models.ScenarioPublication{
		Id:                  uuid.New().String(),
		Rank:                0,
		OrganizationId:      uuid.New().String(),
		ScenarioId:          uuid.New().String(),
		ScenarioIterationId: uuid.New().String(),
		PublicationAction:   0,
		CreatedAt:           time.Now(),
	}

	transaction := new(mocks.Transaction)

	spr := new(mocks.ScenarioPublicationRepository)
	spr.On("CreateScenarioPublication", transaction, createScenarioInput, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(nil)

	spr.On("GetScenarioPublicationById", transaction, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(scenarioPublication, nil)

	swr := new(mocks.ScenarioWriteRepository)
	swr.On("UpdateScenarioLiveIterationId", transaction, scenarioAndIteration.Scenario.Id, (*string)(nil)).
		Return(nil)

	publisher := ScenarioPublisher{
		ScenarioPublicationsRepository: spr,
		ScenarioWriteRepository:        swr,
	}

	publications, err := publisher.PublishOrUnpublishIteration(transaction, scenarioAndIteration, models.Unpublish)
	assert.NoError(t, err)
	assert.Equal(t, []models.ScenarioPublication{scenarioPublication}, publications)

	spr.AssertExpectations(t)
	swr.AssertExpectations(t)
}

func TestScenarioPublisher_PublishOrUnpublishIteration_unpublish_CreateScenarioPublication_error(t *testing.T) {
	Iteration := models.ScenarioIteration{
		Id: uuid.New().String(),
	}

	scenario := models.Scenario{
		OrganizationId: uuid.New().String(),
		Id:             uuid.New().String(),
		LiveVersionID:  utils.Ptr(Iteration.Id),
	}

	scenarioAndIteration := ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: Iteration,
	}

	createScenarioInput := models.CreateScenarioPublicationInput{
		OrganizationId:      scenarioAndIteration.Scenario.OrganizationId,
		ScenarioId:          scenarioAndIteration.Scenario.Id,
		ScenarioIterationId: *scenarioAndIteration.Scenario.LiveVersionID,
		PublicationAction:   models.Unpublish,
	}

	transaction := new(mocks.Transaction)

	spr := new(mocks.ScenarioPublicationRepository)
	spr.On("CreateScenarioPublication", transaction, createScenarioInput, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(assert.AnError)

	swr := new(mocks.ScenarioWriteRepository)

	publisher := ScenarioPublisher{
		ScenarioPublicationsRepository: spr,
		ScenarioWriteRepository:        swr,
	}

	_, err := publisher.PublishOrUnpublishIteration(transaction, scenarioAndIteration, models.Unpublish)
	assert.Error(t, err)

	spr.AssertExpectations(t)
	swr.AssertExpectations(t)
}
