package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// roleEnforceSecurity completes the generic security mock with the user
// methods, which managing custom roles never calls.
type roleEnforceSecurity struct {
	*mocks.EnforceSecurity
}

func (roleEnforceSecurity) ReadUser(models.User) error                      { return errUnexpectedCall }
func (roleEnforceSecurity) CreateUser(models.CreateUser) error              { return errUnexpectedCall }
func (roleEnforceSecurity) UpdateUser(models.User, models.UpdateUser) error { return errUnexpectedCall }
func (roleEnforceSecurity) DeleteUser(models.User) error                    { return errUnexpectedCall }
func (roleEnforceSecurity) ListUsers(*uuid.UUID) error                      { return errUnexpectedCall }
func (roleEnforceSecurity) ListTenantUsers(uuid.UUID) error                 { return errUnexpectedCall }
func (roleEnforceSecurity) ManageOrganizationGrant(uuid.UUID, models.User) error {
	return errUnexpectedCall
}

var errUnexpectedCall = errors.New("unexpected call")

func TestUpdateRolePermissionsOnlyChecksAddedPermissions(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")
	current := models.RbacRole{
		Id:    uuid.New(),
		OrgId: orgId,
		Slug:  string(slug),
		// The caller may not hold PHANTOM_DECISION_CREATE: it is kept, not added.
		Permissions: []models.Permission{models.PHANTOM_DECISION_CREATE},
	}
	permissions := []models.Permission{
		models.PHANTOM_DECISION_CREATE,
		models.CASE_READ_WRITE,
	}

	executor := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	userRepository := new(mocks.UserRepository)
	enforceSecurity := new(mocks.EnforceSecurity)
	executorFactory.On("NewExecutor").Return(executor)
	enforceSecurity.On("ManageRoles").Return(nil)
	enforceSecurity.On("OrgId").Return(orgId)
	enforceSecurity.On("Permissions", []models.Permission{models.CASE_READ_WRITE}).Return(nil).Once()
	userRepository.On("GetRoleBySlug", ctx, executor, orgId, slug).Return(current, nil)
	userRepository.On("UpdateRolePermissions", ctx, executor, orgId, slug, permissions).Return(nil).Once()

	usecase := UserUseCase{
		executorFactory:     executorFactory,
		userRepository:      userRepository,
		enforceUserSecurity: roleEnforceSecurity{enforceSecurity},
	}

	_, err := usecase.UpdateRolePermissions(ctx, slug, permissions)

	require.NoError(t, err)
	enforceSecurity.AssertExpectations(t)
	userRepository.AssertExpectations(t)
}

func TestUpdateRolePermissionsRejectsUnheldAndPlatformPermissions(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")
	current := models.RbacRole{Id: uuid.New(), OrgId: orgId, Slug: string(slug)}

	newUsecase := func(enforceSecurity *mocks.EnforceSecurity, userRepository *mocks.UserRepository) *UserUseCase {
		executorFactory := new(mocks.ExecutorFactory)
		executorFactory.On("NewExecutor").Return(new(mocks.Executor))
		enforceSecurity.On("ManageRoles").Return(nil)
		enforceSecurity.On("OrgId").Return(orgId)
		userRepository.On("GetRoleBySlug", ctx, mock.Anything, orgId, slug).Return(current, nil)
		return &UserUseCase{
			executorFactory:     executorFactory,
			userRepository:      userRepository,
			enforceUserSecurity: roleEnforceSecurity{enforceSecurity},
		}
	}

	t.Run("permission not held", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)
		enforceSecurity.On("Permissions", []models.Permission{models.PHANTOM_DECISION_CREATE}).Return(models.ForbiddenError)

		_, err := newUsecase(enforceSecurity, userRepository).UpdateRolePermissions(ctx, slug,
			[]models.Permission{models.PHANTOM_DECISION_CREATE})

		assert.ErrorIs(t, err, models.ForbiddenError)
		userRepository.AssertNotCalled(t, "UpdateRolePermissions", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("platform permission", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)

		_, err := newUsecase(enforceSecurity, userRepository).UpdateRolePermissions(ctx, slug,
			[]models.Permission{models.ANY_ORGANIZATION_ID_IN_CONTEXT})

		assert.ErrorIs(t, err, models.BadParameterError)
		enforceSecurity.AssertNotCalled(t, "Permissions", mock.Anything)
		userRepository.AssertNotCalled(t, "UpdateRolePermissions", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}
