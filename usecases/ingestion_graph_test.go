package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
)

type graphIngestionMocks struct {
	graph       *mocks.GraphIncrementalRepository
	relations   *mocks.GraphRelationRepository
	features    *mocks.FeatureAccessReader
	transaction *mocks.Transaction
	savepoint   *mocks.Transaction
}

// ingestionGraphUsecase wires the usecase down to the four dependencies the graph path touches. The
// transaction is a mock rather than a stub because the savepoint boundary — Begin, then Commit or
// Rollback — is the behaviour under test.
func ingestionGraphUsecase(t *testing.T, access models.FeatureAccess) (IngestionUseCase, graphIngestionMocks) {
	t.Helper()

	// Each test gets its own organization, so the package-level field cache cannot carry a result
	// from one into another.
	graph := new(mocks.GraphIncrementalRepository)
	relations := new(mocks.GraphRelationRepository)

	dataModel := new(mocks.DataModelRepository)
	dataModel.On("GetDataModel", mock.Anything, mock.Anything, mock.Anything, false, true).
		Return(amlDataModel(), nil)

	features := new(mocks.FeatureAccessReader)
	features.On("GetOrganizationFeatureAccess", mock.Anything, mock.Anything, (*models.UserId)(nil)).
		Return(models.OrganizationFeatureAccess{GraphExploration: access}, nil)

	savepoint := new(mocks.Transaction)
	transaction := new(mocks.Transaction)
	transaction.On("Begin", mock.Anything).Return(repositories.Transaction(savepoint), nil)

	return IngestionUseCase{
		executorFactory:            executor_factory.NewExecutorFactoryStub(),
		dataModelRepository:        dataModel,
		featureAccessReader:        features,
		graphRelationRepository:    relations,
		graphIncrementalRepository: graph,
	}, graphIngestionMocks{
		graph:       graph,
		relations:   relations,
		features:    features,
		transaction: transaction,
		savepoint:   savepoint,
	}
}

func graphIngestionResults(objectIds ...string) models.IngestionResults {
	results := make(models.IngestionResults, len(objectIds))
	for _, objectId := range objectIds {
		results[objectId] = models.IngestionResult{NewInternalId: objectId + "-internal"}
	}
	return results
}

func TestIngestionGraph_WritesTheIngestedRecordsInASavepoint(t *testing.T) {
	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, nil)

	// Sorted, and passed to both statements: the retraction has to consider exactly the records the
	// upsert restated, or a field emptied on one of them would keep its old value.
	expected := []string{"acc-1", "acc-2"}
	m.graph.On("UpsertGraphRows", mock.Anything, repositories.Executor(m.savepoint), "accounts",
		mock.Anything, expected).Return(int64(4), nil)
	m.graph.On("RetractGraphRows", mock.Anything, repositories.Executor(m.savepoint), "accounts",
		mock.Anything, expected).Return(int64(0), nil)
	m.savepoint.On("Commit", mock.Anything).Return(nil)

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-2", "acc-1"))

	m.graph.AssertExpectations(t)
	m.savepoint.AssertExpectations(t)
	m.savepoint.AssertNotCalled(t, "Rollback", mock.Anything)
}

func TestIngestionGraph_RetractsAfterUpserting(t *testing.T) {
	// The retraction decides what to delete by re-reading the live version, so it has to run second:
	// swapped, it would look at rows the upsert had not yet restated.
	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, nil)

	var order []string
	m.graph.On("UpsertGraphRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "upsert") }).Return(int64(1), nil)
	m.graph.On("RetractGraphRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { order = append(order, "retract") }).Return(int64(0), nil)
	m.savepoint.On("Commit", mock.Anything).Return(nil)

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	assert.Equal(t, []string{"upsert", "retract"}, order)
}

// TestIngestionGraph_AFailedWriteRollsBackToTheSavepoint is the test that matters most here. The
// rebuild job swaps the table in by dropping it and renaming over it, so a write racing that swap
// errors — and without the savepoint that error would abort the ingestion transaction with it,
// failing a customer's request over a stale derived table.
func TestIngestionGraph_AFailedWriteRollsBackToTheSavepoint(t *testing.T) {
	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, nil)

	m.graph.On("UpsertGraphRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New(`relation "_graph" does not exist`))
	m.savepoint.On("Rollback", mock.Anything).Return(nil)

	// The absence of a panic or a propagated error is the assertion: maintainGraphRows returns
	// nothing, so ingestion carries on regardless of what happened here.
	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	m.savepoint.AssertCalled(t, "Rollback", mock.Anything)
	m.savepoint.AssertNotCalled(t, "Commit", mock.Anything)
	m.graph.AssertNotCalled(t, "RetractGraphRows", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func TestIngestionGraph_RollsBackWhenTheRetractionFails(t *testing.T) {
	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, nil)

	m.graph.On("UpsertGraphRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(2), nil)
	m.graph.On("RetractGraphRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("statement timeout"))
	m.savepoint.On("Rollback", mock.Anything).Return(nil)

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	// Both statements go back together: an upsert kept without its retraction would leave the value a
	// record no longer has looking live.
	m.savepoint.AssertCalled(t, "Rollback", mock.Anything)
	m.savepoint.AssertNotCalled(t, "Commit", mock.Anything)
}

func TestIngestionGraph_DoesNothingWithoutTheEntitlement(t *testing.T) {
	uc, m := ingestionGraphUsecase(t, models.Restricted)

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	// Not even the transaction is touched: an organization that cannot walk the graph should pay
	// nothing on its ingestion path for one.
	m.transaction.AssertNotCalled(t, "Begin", mock.Anything)
	m.relations.AssertNotCalled(t, "ListGraphRelations", mock.Anything, mock.Anything, mock.Anything)
	m.graph.AssertExpectations(t)
}

func TestIngestionGraph_DoesNothingWhenNoVersionWasWritten(t *testing.T) {
	// A payload older than the stored version is dropped during the merge, so it never reaches the
	// results map — and there is nothing for the graph to restate.
	uc, m := ingestionGraphUsecase(t, models.Allowed)

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], models.IngestionResults{})

	m.features.AssertNotCalled(t, "GetOrganizationFeatureAccess", mock.Anything, mock.Anything, mock.Anything)
	m.transaction.AssertNotCalled(t, "Begin", mock.Anything)
}

func TestIngestionGraph_SurvivesAnUnreadableRelationList(t *testing.T) {
	// The relations live in the marble database, which the ingestion transaction is not talking to.
	// Failing to reach it must cost freshness, not the ingestion.
	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{}, errors.New("connection refused"))

	uc.maintainGraphRows(context.Background(), m.transaction, uuid.New(),
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	m.transaction.AssertNotCalled(t, "Begin", mock.Anything)
	m.graph.AssertExpectations(t)
}

func TestIngestionGraph_PassesTheFieldsTheBuildWouldIndex(t *testing.T) {
	// The two writers have to agree on which fields the table carries, not just on how their values
	// render, so the field set comes from the same function the build uses.
	orgId := uuid.New()
	relation := graphTestRelation("same_swift", "accounts", "swift", "transactions", "sender_iban")

	uc, m := ingestionGraphUsecase(t, models.Allowed)
	m.relations.On("ListGraphRelations", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.GraphRelation{relation}, nil)

	expected := models.GraphIndexedFields(amlDataModel(), []models.GraphRelation{relation})["accounts"]
	require.NotEmpty(t, expected)

	m.graph.On("UpsertGraphRows", mock.Anything, mock.Anything, "accounts", expected, mock.Anything).
		Return(int64(1), nil)
	m.graph.On("RetractGraphRows", mock.Anything, mock.Anything, "accounts", expected, mock.Anything).
		Return(int64(0), nil)
	m.savepoint.On("Commit", mock.Anything).Return(nil)

	uc.maintainGraphRows(context.Background(), m.transaction, orgId,
		amlDataModel().Tables["accounts"], graphIngestionResults("acc-1"))

	m.graph.AssertExpectations(t)
}
