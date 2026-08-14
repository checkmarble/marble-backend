package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

func graphRelationUsecase(repo *mocks.GraphRelationRepository, orgId uuid.UUID) GraphRelationUsecase {
	enforceSecurity := new(mocks.EnforceSecurity)
	enforceSecurity.On("OrgId").Return(orgId)
	enforceSecurity.On("ReadDataModel").Return(nil)
	enforceSecurity.On("WriteDataModel", mock.Anything).Return(nil)
	enforceSecurity.On("ReadOrganization", mock.Anything).Return(nil)

	dataModelRepository := new(mocks.DataModelRepository)
	dataModelRepository.On("GetDataModel", mock.Anything, mock.Anything, mock.Anything, false, true).
		Return(amlDataModel(), nil)

	return GraphRelationUsecase{
		enforceSecurity:         enforceSecurity,
		executorFactory:         executor_factory.NewExecutorFactoryStub(),
		dataModelRepository:     dataModelRepository,
		graphRelationRepository: repo,
	}
}

func createGraphRelationInput(orgId uuid.UUID) models.CreateGraphRelation {
	return models.CreateGraphRelation{
		OrgId:     orgId,
		Label:     "same_iban",
		LeftType:  "accounts",
		LeftField: "iban",
		RightType: "accounts", RightField: "iban",
	}
}

func TestCreateGraphRelation_AcceptsEndpointsPresentInTheDataModel(t *testing.T) {
	orgId := uuid.New()
	repo := new(mocks.GraphRelationRepository)
	created := models.GraphRelation{Id: pure_utils.NewId(), OrgId: orgId, Label: "same_iban"}
	repo.On("CreateGraphRelation", mock.Anything, mock.Anything, mock.AnythingOfType("models.CreateGraphRelation")).
		Return(created, nil)

	relation, err := graphRelationUsecase(repo, orgId).
		CreateGraphRelation(context.Background(), createGraphRelationInput(orgId))

	require.NoError(t, err)
	assert.Equal(t, created, relation)
	repo.AssertExpectations(t)
}

func TestCreateGraphRelation_RejectsAnEndpointAbsentFromTheDataModel(t *testing.T) {
	// A relation naming a table or field that does not exist would be silently skipped by every
	// later walk, so it must not be storable in the first place.
	tests := []struct {
		name  string
		input func(models.CreateGraphRelation) models.CreateGraphRelation
	}{
		{
			name: "unknown table on the left",
			input: func(in models.CreateGraphRelation) models.CreateGraphRelation {
				in.LeftType = "gadgets"
				return in
			},
		},
		{
			name: "unknown field on the left",
			input: func(in models.CreateGraphRelation) models.CreateGraphRelation {
				in.LeftField = "nope"
				return in
			},
		},
		{
			name: "unknown table on the right",
			input: func(in models.CreateGraphRelation) models.CreateGraphRelation {
				in.RightType = "gadgets"
				return in
			},
		},
		{
			name: "unknown field on the right",
			input: func(in models.CreateGraphRelation) models.CreateGraphRelation {
				in.RightField = "nope"
				return in
			},
		},
	}

	orgId := uuid.New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mocks.GraphRelationRepository)

			_, err := graphRelationUsecase(repo, orgId).CreateGraphRelation(context.Background(),
				tt.input(createGraphRelationInput(uuid.New())))

			assert.ErrorIs(t, err, models.BadParameterError)
			repo.AssertNotCalled(t, "CreateGraphRelation", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCreateGraphRelation_MintsAGroupForARelationJoiningNone(t *testing.T) {
	// A relation created without a group is a group of its own. The id has to be minted here
	// rather than left to the caller or the database, because it is the identity the graph walk
	// collapses connectors onto.
	orgId := uuid.New()
	repo := new(mocks.GraphRelationRepository)

	var stored models.CreateGraphRelation

	repo.On("CreateGraphRelation", mock.Anything, mock.Anything, mock.AnythingOfType("models.CreateGraphRelation")).
		Run(func(args mock.Arguments) {
			stored = args.Get(2).(models.CreateGraphRelation)
		}).
		Return(models.GraphRelation{}, nil)

	_, err := graphRelationUsecase(repo, orgId).
		CreateGraphRelation(context.Background(), createGraphRelationInput(orgId))

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, stored.GroupId)
	repo.AssertNotCalled(t, "GetGraphRelationGroupLabel", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateGraphRelation_JoinsAnExistingGroupUnderItsLabel(t *testing.T) {
	// Joining a group keeps the group id the caller asked for. Two groups may well be labelled
	// the same, so the label alone could never have identified which one was meant.
	orgId := uuid.New()
	groupId := pure_utils.NewId()

	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelationGroupLabel", mock.Anything, mock.Anything, orgId, groupId).
		Return("same_iban", nil)

	var stored models.CreateGraphRelation

	repo.On("CreateGraphRelation", mock.Anything, mock.Anything, mock.AnythingOfType("models.CreateGraphRelation")).
		Run(func(args mock.Arguments) {
			stored = args.Get(2).(models.CreateGraphRelation)
		}).
		Return(models.GraphRelation{}, nil)

	input := createGraphRelationInput(orgId)
	input.GroupId = groupId

	_, err := graphRelationUsecase(repo, orgId).CreateGraphRelation(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, groupId, stored.GroupId)
	assert.Equal(t, "same_iban", stored.Label)
	repo.AssertExpectations(t)
}

func TestCreateGraphRelation_RefusesALabelTheJoinedGroupDoesNotCarry(t *testing.T) {
	// The label belongs to the group, so a relation cannot bring its own. Taking the group's and
	// discarding what was sent would leave the caller believing its label took.
	orgId := uuid.New()
	groupId := pure_utils.NewId()

	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelationGroupLabel", mock.Anything, mock.Anything, orgId, groupId).
		Return("same_iban", nil)

	input := createGraphRelationInput(orgId)
	input.GroupId = groupId
	input.Label = "something_else"

	_, err := graphRelationUsecase(repo, orgId).CreateGraphRelation(context.Background(), input)

	assert.ErrorIs(t, err, models.BadParameterError)
	repo.AssertNotCalled(t, "CreateGraphRelation", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateGraphRelation_RefusesAGroupOfAnotherOrganization(t *testing.T) {
	// The group lookup is scoped by organization, so another organization's group reads as one
	// that does not exist — which is what it should look like from here.
	orgId := uuid.New()
	groupId := pure_utils.NewId()

	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelationGroupLabel", mock.Anything, mock.Anything, orgId, groupId).
		Return("", errors.Wrap(models.NotFoundError, "provided group does not exist"))

	input := createGraphRelationInput(orgId)
	input.GroupId = groupId

	_, err := graphRelationUsecase(repo, orgId).CreateGraphRelation(context.Background(), input)

	assert.ErrorIs(t, err, models.NotFoundError)
	repo.AssertNotCalled(t, "CreateGraphRelation", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateGraphRelation_ReportsADuplicateAsAConflict(t *testing.T) {
	orgId := uuid.New()

	repo := new(mocks.GraphRelationRepository)
	repo.On("CreateGraphRelation", mock.Anything, mock.Anything, mock.AnythingOfType("models.CreateGraphRelation")).
		Return(models.GraphRelation{}, &pgconn.PgError{Code: pgerrcode.UniqueViolation})

	_, err := graphRelationUsecase(repo, orgId).
		CreateGraphRelation(context.Background(), createGraphRelationInput(uuid.New()))

	assert.ErrorIs(t, err, models.ConflictError)
}

func TestDeleteGraphRelation_RefusesAnotherOrganizationsRelation(t *testing.T) {
	other := models.GraphRelation{Id: pure_utils.NewId(), OrgId: uuid.New(), Label: "same_iban"}
	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelation", mock.Anything, mock.Anything, other.Id).Return(other, nil)

	err := graphRelationUsecase(repo, uuid.New()).
		DeleteGraphRelation(context.Background(), other.Id)

	assert.ErrorIs(t, err, models.ForbiddenError)
	repo.AssertNotCalled(t, "DeleteGraphRelation", mock.Anything, mock.Anything, mock.Anything)
}

func TestDeleteGraphRelation_DeletesItsOwnRelation(t *testing.T) {
	orgId := uuid.New()
	own := models.GraphRelation{Id: pure_utils.NewId(), OrgId: orgId, Label: "same_iban"}
	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelation", mock.Anything, mock.Anything, own.Id).Return(own, nil)
	repo.On("DeleteGraphRelation", mock.Anything, mock.Anything, own.Id).Return(nil)

	err := graphRelationUsecase(repo, orgId).DeleteGraphRelation(context.Background(), own.Id)

	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteGraphRelation_ReportsAnUnknownRelationAsNotFound(t *testing.T) {
	id := pure_utils.NewId()
	repo := new(mocks.GraphRelationRepository)
	repo.On("GetGraphRelation", mock.Anything, mock.Anything, id).
		Return(models.GraphRelation{}, errors.Wrap(models.NotFoundError, "no such graph relation"))

	err := graphRelationUsecase(repo, uuid.New()).
		DeleteGraphRelation(context.Background(), id)

	assert.ErrorIs(t, err, models.NotFoundError)
}

func TestListGraphRelations_ReturnsOnlyTheCallersOrganization(t *testing.T) {
	orgId := uuid.New()
	repo := new(mocks.GraphRelationRepository)
	repo.On("ListGraphRelations", mock.Anything, mock.Anything, orgId).
		Return([]models.GraphRelation{{Id: pure_utils.NewId(), OrgId: orgId, Label: "same_iban"}}, nil)

	relations, err := graphRelationUsecase(repo, orgId).ListGraphRelations(context.Background())

	require.NoError(t, err)
	require.Len(t, relations, 1)
	assert.Equal(t, "same_iban", relations[0].Label)
}
