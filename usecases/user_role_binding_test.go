package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResolveUserRoleBindingsResolvesCustomRoleSlug(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")
	role := models.RbacRole{
		Id:             uuid.New(),
		OrgId:          orgId,
		Slug:           string(slug),
		Permissions:    []models.Permission{models.CASE_READ_WRITE},
		HiddenSections: []models.AppSection{models.SETTINGS},
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
	assert.Equal(t, role.HiddenSections, resolved[0].HiddenSections)
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
func (roleEnforceSecurity) ListUserGrants() error { return errUnexpectedCall }

var errUnexpectedCall = errors.New("unexpected call")

func TestUpdateRoleSections(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")

	for _, tt := range []struct {
		name     string
		sections map[models.AppSection]bool
		hidden   []models.AppSection
	}{
		{name: "only false entries stored", sections: map[models.AppSection]bool{
			models.SETTINGS: false, models.SCENARIOS: true,
		}, hidden: []models.AppSection{models.SETTINGS}},
		{name: "empty map clears restrictions", sections: map[models.AppSection]bool{}, hidden: []models.AppSection{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			executor := new(mocks.Executor)
			factory := new(mocks.ExecutorFactory)
			repo := new(mocks.UserRepository)
			security := new(mocks.EnforceSecurity)
			role := models.RbacRole{OrgId: orgId, Slug: string(slug), HiddenSections: tt.hidden}
			factory.On("NewExecutor").Return(executor).Once()
			security.On("ManageRoles").Return(nil).Once()
			security.On("OrgId").Return(orgId)
			repo.On("UpdateRoleHiddenSections", ctx, executor, orgId, slug, tt.hidden).Return(nil).Once()
			repo.On("GetRoleBySlug", ctx, executor, orgId, slug).Return(role, nil).Once()
			usecase := UserUseCase{
				executorFactory:     factory,
				userRepository:      repo,
				enforceUserSecurity: roleEnforceSecurity{security},
				featureAccessReader: newRolesFeatureAccessReader(models.Allowed),
			}

			updated, err := usecase.UpdateRoleSections(ctx, slug, tt.sections)

			require.NoError(t, err)
			assert.Equal(t, role, updated)
			security.AssertExpectations(t)
			repo.AssertExpectations(t)
			factory.AssertExpectations(t)
		})
	}
}

func TestUpdateRoleSectionsRejectsInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name          string
		slug          models.Role
		sections      map[models.AppSection]bool
		securityError error
		wantError     error
	}{
		{name: "forbidden", slug: "org/custom", securityError: models.ForbiddenError, wantError: models.ForbiddenError},
		{name: "native role", slug: models.ANALYST, wantError: models.BadParameterError},
		{name: "invalid slug", slug: "org/invalid-role", wantError: models.BadParameterError},
		{name: "unknown section", slug: "org/custom", sections: map[models.AppSection]bool{"unknown": true}, wantError: models.BadParameterError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			security := new(mocks.EnforceSecurity)
			security.On("ManageRoles").Return(tt.securityError).Once()
			security.On("OrgId").Return(uuid.New()).Maybe()
			usecase := UserUseCase{
				enforceUserSecurity: roleEnforceSecurity{security},
				featureAccessReader: newRolesFeatureAccessReader(models.Allowed),
			}

			_, err := usecase.UpdateRoleSections(context.Background(), tt.slug, tt.sections)

			assert.ErrorIs(t, err, tt.wantError)
			security.AssertExpectations(t)
		})
	}
}

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
		featureAccessReader: newRolesFeatureAccessReader(models.Allowed),
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
			featureAccessReader: newRolesFeatureAccessReader(models.Allowed),
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

func TestGetRolesDoesNotRequireManageRoles(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	roles := []models.RbacRole{{Id: uuid.New(), OrgId: orgId, Slug: "org/custom.name"}}

	executor := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	userRepository := new(mocks.UserRepository)
	enforceSecurity := new(mocks.EnforceSecurity)
	executorFactory.On("NewExecutor").Return(executor)
	enforceSecurity.On("OrgId").Return(orgId)
	enforceSecurity.On("ReadOrganization", orgId).Return(nil).Once()
	userRepository.On("ListRoles", ctx, executor, orgId).Return(roles, nil).Once()

	usecase := UserUseCase{
		executorFactory:     executorFactory,
		userRepository:      userRepository,
		enforceUserSecurity: roleEnforceSecurity{enforceSecurity},
	}

	listed, err := usecase.GetRoles(ctx)

	require.NoError(t, err)
	assert.Equal(t, roles, listed)
	enforceSecurity.AssertNotCalled(t, "ManageRoles")
	enforceSecurity.AssertExpectations(t)
	userRepository.AssertExpectations(t)
}

func TestDeleteRole(t *testing.T) {
	ctx := context.Background()
	orgId := uuid.New()
	slug := models.Role("org/custom.name")
	role := models.RbacRole{Id: uuid.New(), OrgId: orgId, Slug: string(slug)}

	newUsecase := func(enforceSecurity *mocks.EnforceSecurity, userRepository *mocks.UserRepository) (*UserUseCase, *mocks.Transaction) {
		executor := new(mocks.Executor)
		tx := new(mocks.Transaction)
		executorFactory := new(mocks.ExecutorFactory)
		transactionFactory := &mocks.TransactionFactory{TxMock: tx}
		executorFactory.On("NewExecutor").Return(executor)
		transactionFactory.On("Transaction", ctx, mock.Anything).Return(nil)
		enforceSecurity.On("OrgId").Return(orgId)

		return &UserUseCase{
			executorFactory:     executorFactory,
			transactionFactory:  transactionFactory,
			userRepository:      userRepository,
			enforceUserSecurity: roleEnforceSecurity{enforceSecurity},
			featureAccessReader: newRolesFeatureAccessReader(models.Allowed),
		}, tx
	}

	t.Run("unused role", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)
		enforceSecurity.On("ManageRoles").Return(nil)
		usecase, tx := newUsecase(enforceSecurity, userRepository)
		userRepository.On("GetRoleBySlug", ctx, mock.Anything, orgId, slug).Return(role, nil)
		userRepository.On("DeleteRole", ctx, tx, orgId, role.Id).Return(nil).Once()

		err := usecase.DeleteRole(ctx, slug)

		require.NoError(t, err)
		userRepository.AssertExpectations(t)
	})

	t.Run("role still bound", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)
		enforceSecurity.On("ManageRoles").Return(nil)
		usecase, tx := newUsecase(enforceSecurity, userRepository)
		userRepository.On("GetRoleBySlug", ctx, mock.Anything, orgId, slug).Return(role, nil)
		userRepository.On("DeleteRole", ctx, tx, orgId, role.Id).
			Return(&pgconn.PgError{Code: pgerrcode.ForeignKeyViolation}).Once()

		err := usecase.DeleteRole(ctx, slug)

		assert.ErrorIs(t, err, models.ConflictError)
	})

	t.Run("unknown role", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)
		enforceSecurity.On("ManageRoles").Return(nil)
		usecase, _ := newUsecase(enforceSecurity, userRepository)
		userRepository.On("GetRoleBySlug", ctx, mock.Anything, orgId, slug).Return(models.RbacRole{}, models.NotFoundError)

		err := usecase.DeleteRole(ctx, slug)

		assert.ErrorIs(t, err, models.NotFoundError)
		userRepository.AssertNotCalled(t, "DeleteRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("without MANAGE_ROLES", func(t *testing.T) {
		enforceSecurity, userRepository := new(mocks.EnforceSecurity), new(mocks.UserRepository)
		enforceSecurity.On("ManageRoles").Return(models.ForbiddenError)
		usecase, _ := newUsecase(enforceSecurity, userRepository)

		err := usecase.DeleteRole(ctx, slug)

		assert.ErrorIs(t, err, models.ForbiddenError)
		userRepository.AssertNotCalled(t, "DeleteRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestManagingRolesRequiresRolesFeature(t *testing.T) {
	ctx := context.Background()
	enforceSecurity := new(mocks.EnforceSecurity)
	userRepository := new(mocks.UserRepository)
	enforceSecurity.On("ManageRoles").Return(nil)
	enforceSecurity.On("OrgId").Return(uuid.New())

	usecase := UserUseCase{
		userRepository:      userRepository,
		enforceUserSecurity: roleEnforceSecurity{enforceSecurity},
		featureAccessReader: newRolesFeatureAccessReader(models.Restricted),
	}

	_, err := usecase.CreateRole(ctx, "org/reviewer", "Reviewer")
	assert.ErrorIs(t, err, models.ForbiddenError)

	_, err = usecase.UpdateRolePermissions(ctx, "org/reviewer", []models.Permission{models.CASE_READ_WRITE})
	assert.ErrorIs(t, err, models.ForbiddenError)

	err = usecase.DeleteRole(ctx, "org/reviewer")
	assert.ErrorIs(t, err, models.ForbiddenError)

	_, err = usecase.UpdateRoleSections(ctx, "org/reviewer", map[models.AppSection]bool{models.SETTINGS: false})
	assert.ErrorIs(t, err, models.ForbiddenError)

	userRepository.AssertNotCalled(t, "CreateRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	userRepository.AssertNotCalled(t, "UpdateRolePermissions", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	userRepository.AssertNotCalled(t, "DeleteRole", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	userRepository.AssertNotCalled(t, "UpdateRoleHiddenSections", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
