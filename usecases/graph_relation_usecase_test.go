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
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type fakeGraphRelationRepository struct {
	relations []models.GraphRelation
	createErr error
	deleted   []uuid.UUID
}

func (repo *fakeGraphRelationRepository) ListGraphRelations(_ context.Context, _ repositories.Executor, orgId uuid.UUID) ([]models.GraphRelation, error) {
	out := make([]models.GraphRelation, 0, len(repo.relations))
	for _, relation := range repo.relations {
		if relation.OrgId == orgId {
			out = append(out, relation)
		}
	}
	return out, nil
}

func (repo *fakeGraphRelationRepository) GetGraphRelation(_ context.Context, _ repositories.Executor, id uuid.UUID) (models.GraphRelation, error) {
	for _, relation := range repo.relations {
		if relation.Id == id {
			return relation, nil
		}
	}
	return models.GraphRelation{}, errors.Wrap(models.NotFoundError, "no such graph relation")
}

func (repo *fakeGraphRelationRepository) CreateGraphRelation(_ context.Context, _ repositories.Executor, input models.CreateGraphRelation) (models.GraphRelation, error) {
	if repo.createErr != nil {
		return models.GraphRelation{}, repo.createErr
	}

	relation := models.GraphRelation{
		Id:         pure_utils.NewId(),
		OrgId:      input.OrgId,
		Label:      input.Label,
		LeftType:   input.LeftType,
		LeftField:  input.LeftField,
		RightType:  input.RightType,
		RightField: input.RightField,
	}
	repo.relations = append(repo.relations, relation)

	return relation, nil
}

func (repo *fakeGraphRelationRepository) DeleteGraphRelation(
	_ context.Context, _ repositories.Executor, id uuid.UUID,
) error {
	repo.deleted = append(repo.deleted, id)
	return nil
}

func graphRelationUsecase(repo *fakeGraphRelationRepository, orgId uuid.UUID) GraphRelationUsecase {
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
	repo := &fakeGraphRelationRepository{}

	relation, err := graphRelationUsecase(repo, orgId).
		CreateGraphRelation(context.Background(), createGraphRelationInput(orgId))

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, relation.Id)
	assert.Equal(t, orgId, relation.OrgId)
	assert.Len(t, repo.relations, 1)
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
			repo := &fakeGraphRelationRepository{}

			_, err := graphRelationUsecase(repo, orgId).CreateGraphRelation(context.Background(),
				tt.input(createGraphRelationInput(uuid.New())))

			assert.ErrorIs(t, err, models.BadParameterError)
			assert.Empty(t, repo.relations, "nothing is stored when validation fails")
		})
	}
}

func TestCreateGraphRelation_ReportsADuplicateAsAConflict(t *testing.T) {
	orgId := uuid.New()

	repo := &fakeGraphRelationRepository{
		createErr: &pgconn.PgError{Code: pgerrcode.UniqueViolation},
	}

	_, err := graphRelationUsecase(repo, orgId).
		CreateGraphRelation(context.Background(), createGraphRelationInput(uuid.New()))

	assert.ErrorIs(t, err, models.ConflictError)
}

func TestDeleteGraphRelation_RefusesAnotherOrganizationsRelation(t *testing.T) {
	other := models.GraphRelation{Id: pure_utils.NewId(), OrgId: uuid.New(), Label: "same_iban"}
	repo := &fakeGraphRelationRepository{relations: []models.GraphRelation{other}}

	err := graphRelationUsecase(repo, uuid.New()).
		DeleteGraphRelation(context.Background(), other.Id)

	assert.ErrorIs(t, err, models.ForbiddenError)
	assert.Empty(t, repo.deleted, "the relation is left alone")
}

func TestDeleteGraphRelation_DeletesItsOwnRelation(t *testing.T) {
	orgId := uuid.New()
	own := models.GraphRelation{Id: pure_utils.NewId(), OrgId: orgId, Label: "same_iban"}
	repo := &fakeGraphRelationRepository{relations: []models.GraphRelation{own}}

	err := graphRelationUsecase(repo, orgId).DeleteGraphRelation(context.Background(), own.Id)

	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{own.Id}, repo.deleted)
}

func TestDeleteGraphRelation_ReportsAnUnknownRelationAsNotFound(t *testing.T) {
	repo := &fakeGraphRelationRepository{}

	err := graphRelationUsecase(repo, uuid.New()).
		DeleteGraphRelation(context.Background(), pure_utils.NewId())

	assert.ErrorIs(t, err, models.NotFoundError)
}

func TestListGraphRelations_ReturnsOnlyTheCallersOrganization(t *testing.T) {
	orgId := uuid.New()
	repo := &fakeGraphRelationRepository{relations: []models.GraphRelation{
		{Id: pure_utils.NewId(), OrgId: orgId, Label: "same_iban"},
		{Id: pure_utils.NewId(), OrgId: uuid.New(), Label: "same_ip"},
	}}

	relations, err := graphRelationUsecase(repo, orgId).ListGraphRelations(context.Background())

	require.NoError(t, err)
	require.Len(t, relations, 1)
	assert.Equal(t, "same_iban", relations[0].Label)
}
