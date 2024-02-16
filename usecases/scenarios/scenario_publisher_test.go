package scenarios

import (
	"context"
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

	scenarioAndIteration := models.ScenarioAndIteration{
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

	transaction := new(mocks.Executor)
	ctx := context.Background()

	repo := new(mocks.ScenarioPublisherRepository)
	repo.On("UpdateScenarioLiveIterationId", ctx, transaction, scenarioAndIteration.Scenario.Id, (*string)(nil)).Return(nil)

	spr := new(mocks.ScenarioPublicationRepository)
	spr.On("CreateScenarioPublication", ctx, transaction, createScenarioInput, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(nil)

	spr.On("GetScenarioPublicationById", ctx, transaction, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(scenarioPublication, nil)

	publisher := ScenarioPublisher{
		Repository:                     repo,
		ScenarioPublicationsRepository: spr,
	}

	publications, err := publisher.PublishOrUnpublishIteration(
		ctx, transaction, scenarioAndIteration, models.Unpublish)
	assert.NoError(t, err)
	assert.Equal(t, []models.ScenarioPublication{scenarioPublication}, publications)

	spr.AssertExpectations(t)
	repo.AssertExpectations(t)
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

	scenarioAndIteration := models.ScenarioAndIteration{
		Scenario:  scenario,
		Iteration: Iteration,
	}

	createScenarioInput := models.CreateScenarioPublicationInput{
		OrganizationId:      scenarioAndIteration.Scenario.OrganizationId,
		ScenarioId:          scenarioAndIteration.Scenario.Id,
		ScenarioIterationId: *scenarioAndIteration.Scenario.LiveVersionID,
		PublicationAction:   models.Unpublish,
	}

	transaction := new(mocks.Executor)
	ctx := context.Background()

	spr := new(mocks.ScenarioPublicationRepository)
	spr.On("CreateScenarioPublication", ctx, transaction, createScenarioInput, mock.MatchedBy(func(id string) bool {
		_, err := uuid.Parse(id)
		return err == nil
	})).Return(assert.AnError)

	swr := new(mocks.ScenarioRepository)

	repo := new(mocks.ScenarioPublisherRepository)
	repo.On("UpdateScenarioLiveIterationId", ctx, transaction, scenarioAndIteration.Scenario.Id, (*string)(nil)).Return(nil)

	publisher := ScenarioPublisher{
		Repository:                     repo,
		ScenarioPublicationsRepository: spr,
	}

	_, err := publisher.PublishOrUnpublishIteration(context.Background(), transaction, scenarioAndIteration, models.Unpublish)
	assert.Error(t, err)

	spr.AssertExpectations(t)
	swr.AssertExpectations(t)
}
