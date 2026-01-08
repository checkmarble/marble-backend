package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestParseOpenSanctionEntityIdToMarbleObject(t *testing.T) {
	ctx := context.Background()
	repoMock := &mocks.CaseRepository{}
	usecase := &CaseUseCase{
		repository: repoMock,
	}

	configId := uuid.New()
	var exec repositories.Executor // Nil executor for testing

	t.Run("successfully parses multiple matches with overlapping prefix lengths", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(models.ContinuousScreeningConfig{
			Id:          configId,
			ObjectTypes: []string{"User", "UserProfile", "Account"},
		}, nil).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
			Matches: []models.ContinuousScreeningMatch{
				{OpenSanctionEntityId: "marble_UserProfile_user123"},
				{OpenSanctionEntityId: "marble_User_user456"},
				{OpenSanctionEntityId: "marble_Account_acc789"},
			},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.NoError(t, err)
		assert.Equal(t, "UserProfile", continuousScreeningWithMatches.Matches[0].ObjectType)
		assert.Equal(t, "user123", continuousScreeningWithMatches.Matches[0].ObjectId)

		assert.Equal(t, "User", continuousScreeningWithMatches.Matches[1].ObjectType)
		assert.Equal(t, "user456", continuousScreeningWithMatches.Matches[1].ObjectId)

		assert.Equal(t, "Account", continuousScreeningWithMatches.Matches[2].ObjectType)
		assert.Equal(t, "acc789", continuousScreeningWithMatches.Matches[2].ObjectId)

		repoMock.AssertExpectations(t)
	})

	t.Run("returns error when prefix does not match any object type", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(models.ContinuousScreeningConfig{
			Id:          configId,
			ObjectTypes: []string{"User"},
		}, nil).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
			Matches: []models.ContinuousScreeningMatch{
				{OpenSanctionEntityId: "marble_Unknown_123"},
			},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse open sanction entity id to marble object")
		repoMock.AssertExpectations(t)
	})

	t.Run("returns error when GetContinuousScreeningConfig fails", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(
			models.ContinuousScreeningConfig{}, assert.AnError).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not get continuous screening config")
		repoMock.AssertExpectations(t)
	})

	t.Run("successfully handles empty matches list", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(models.ContinuousScreeningConfig{
			Id:          configId,
			ObjectTypes: []string{"User"},
		}, nil).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
			Matches: []models.ContinuousScreeningMatch{},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.NoError(t, err)
		repoMock.AssertExpectations(t)
	})

	t.Run("returns error when OpenSanctionEntityId is empty", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(models.ContinuousScreeningConfig{
			Id:          configId,
			ObjectTypes: []string{"User"},
		}, nil).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
			Matches: []models.ContinuousScreeningMatch{
				{OpenSanctionEntityId: ""},
			},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse open sanction entity id to marble object")
		repoMock.AssertExpectations(t)
	})

	t.Run("returns error when match does not have marble_ prefix", func(t *testing.T) {
		repoMock.On("GetContinuousScreeningConfig", ctx, exec, configId).Return(models.ContinuousScreeningConfig{
			Id:          configId,
			ObjectTypes: []string{"User"},
		}, nil).Once()

		continuousScreeningWithMatches := &models.ContinuousScreeningWithMatches{
			ContinuousScreening: models.ContinuousScreening{
				ContinuousScreeningConfigId: configId,
			},
			Matches: []models.ContinuousScreeningMatch{
				{OpenSanctionEntityId: "something_else_User_123"},
			},
		}

		err := usecase.parseOpenSanctionEntityIdToMarbleObject(ctx, exec, continuousScreeningWithMatches)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse open sanction entity id to marble object")
		repoMock.AssertExpectations(t)
	})
}
