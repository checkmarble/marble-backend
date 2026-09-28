package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUserRoleBindingsResolvesCustomRoleSlug(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")
	role := models.RbacRole{
		Id:          uuid.New(),
		OrgId:       orgId,
		Slug:        string(slug),
		Permissions: []models.Permission{models.CASE_READ_WRITE},
	}
	executor := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	userRepository := new(mocks.UserRepository)
	executorFactory.On("NewExecutor").Return(executor).Once()
	userRepository.On("GetRoleBySlug", ctx, executor, orgId, slug).Return(role, nil).Once()
	usecase := UserUseCase{
		executorFactory: executorFactory,
		userRepository:  userRepository,
	}

	resolved, err := usecase.resolveUserRoleBindings(ctx, orgId, []models.RoleBinding{{Role: slug}})

	require.NoError(t, err)
	require.Len(t, resolved, 1)
	assert.Equal(t, slug, resolved[0].Role)
	require.NotNil(t, resolved[0].CustomRoleId)
	assert.Equal(t, role.Id, *resolved[0].CustomRoleId)
	assert.Equal(t, role.Permissions, resolved[0].Permissions)
	executorFactory.AssertExpectations(t)
	userRepository.AssertExpectations(t)
}
