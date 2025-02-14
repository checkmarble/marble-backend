package usecases

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestParseStringValuesToMap(t *testing.T) {
	table := models.Table{
		Name: "transactions",
		Fields: map[string]models.Field{
			"object_id": {
				DataType: models.String, Nullable: false,
			},
			"updated_at": {DataType: models.Timestamp, Nullable: false},
			"value":      {DataType: models.Float, Nullable: true},
			"status":     {DataType: models.String, Nullable: true},
		},
		LinksToSingle: nil,
	}

	type testCase struct {
		name    string
		columns []string
		values  []string
	}

	OKcases := []testCase{
		{
			name:    "valid case with all fields present",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1", "2020-01-01T00:00:00Z", "1.0", "OK"},
		},
		{
			name:    "valid case with empty status and null value",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1", "2020-01-01T00:00:00Z", "", ""},
		},
		{
			name:    "error case with the other format updated_at (missing T & Z)",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "2023-01-01 00:00:00", "", ""},
		},
	}

	for _, c := range OKcases {
		_, err := parseStringValuesToMap(c.columns, c.values, table)
		if err != nil {
			t.Errorf("Error parsing string values to map: %v", err)
		}
	}

	ErrCases := []testCase{
		{
			name:    "error case with missing object_id",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"", "2020-01-01T00:00:00Z", "", ""},
		},
		{
			name:    "error case with missing updated_at",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "", "", ""},
		},
		{
			name:    "error case with bad format updated_at",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "2023-01-01", "", ""},
		},
		{
			name:    "error case with bad format value",
			columns: []string{"object_id", "updated_at", "value", "status"},
			values:  []string{"1234", "2023-01-01T00:00:00Z", "This is not a number", ""},
		},
	}
	for _, c := range ErrCases {
		_, err := parseStringValuesToMap(c.columns, c.values, table)
		if err == nil {
			t.Errorf("Expected error parsing string values to map: %v", err)
		}
	}
}

type IngestionUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity     *mocks.EnforceSecurity
	executorFactory     executor_factory.ExecutorFactoryStub
	transactionFactory  executor_factory.TransactionFactoryStub
	dataModelRepository *mocks.DataModelRepository

	organizationId string
	dataModel      models.DataModel

	ctx context.Context
}

func (suite *IngestionUsecaseTestSuite) makeUsecase() *IngestionUseCase {
	return &IngestionUseCase{
		transactionFactory:    suite.transactionFactory,
		executorFactory:       suite.executorFactory,
		enforceSecurity:       suite.enforceSecurity,
		ingestionRepository:   &repositories.IngestionRepositoryImpl{},
		dataModelRepository:   suite.dataModelRepository,
		batchIngestionMaxSize: 100,
	}
}

func (suite *IngestionUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)
	suite.dataModelRepository = new(mocks.DataModelRepository)

	suite.organizationId = "org_id"
	suite.dataModel = models.DataModel{
		Tables: map[string]models.Table{
			"transactions": {
				Name: "transactions",
				Fields: map[string]models.Field{
					"object_id": {
						DataType: models.String, Nullable: false, Name: "object_id",
					},
					"updated_at": {DataType: models.Timestamp, Nullable: false, Name: "updated_at"},
					"value":      {DataType: models.Float, Nullable: true, Name: "value"},
					"status":     {DataType: models.String, Nullable: false, Name: "status"},
				},
				LinksToSingle: nil,
			},
		},
	}

	suite.ctx = utils.StoreOpenTelemetryTracerInContext(
		utils.StoreLoggerInContext(context.TODO(), utils.NewLogger("text")),
		&noop.Tracer{})
}

func (suite *IngestionUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	asserts := assert.New(t)
	// Wait here so we are sure to gather the async call to dataModelRepository.BatchInsertEnumValues
	time.Sleep(50 * time.Millisecond)
	asserts.NoError(suite.executorFactory.Mock.ExpectationsWereMet(),
		"ExecutorFactory expectations were not met")
	suite.dataModelRepository.AssertExpectations(t)
	suite.dataModelRepository.AssertExpectations(t)
	suite.enforceSecurity.AssertExpectations(t)
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_nominal_with_previous_version() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	rowIdStr := "17c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId := utils.ByteUuid(rowIdStr)
	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is a previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}).
			AddRow("1", "OK", updAt, 1.0, rowId))
	// update the previous version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`UPDATE "test"."transactions" SET valid_until = $1 WHERE id IN ($2)`)).
		WithArgs("now()", rowIdStr).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// insert the new version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}`))
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting object")
	asserts.Equal(1, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_nominal_no_previous_version() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is no previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}))
	// insert the new version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}`))
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting object")
	asserts.Equal(1, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_nominal_no_previous_version_and_enum() {
	t := suite.T()
	uc := suite.makeUsecase()

	// update the basic data model to include an enum, and use this copy just in this test
	dataModel := suite.dataModel.Copy()
	table := dataModel.Tables["transactions"]
	field := table.Fields["status"]
	field.IsEnum = true
	table.Fields["status"] = field

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(dataModel, nil)

	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is no previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}))
	// insert the new version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	values := make(models.EnumValues)
	values["status"] = map[any]struct{}{"OK": {}}
	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), values, dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}`))

	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting object")
	asserts.Equal(1, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_nominal_with_more_recent_previous_version() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	rowIdStr := "17c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId := utils.ByteUuid(rowIdStr)
	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is a previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}).
			AddRow("1", "OK", updAt.Add(time.Hour), 1.0, rowId))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}`))
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting object")
	asserts.Equal(0, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_nominal_with_previous_version_and_partial_insert() {
	// "status" is missing in the payload, but it can be read from a previous version of the object
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	rowIdStr := "17c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId := utils.ByteUuid(rowIdStr)
	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is a previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}).
			AddRow("1", "OK", updAt, 1.0, rowId))
	// update the previous version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`UPDATE "test"."transactions" SET valid_until = $1 WHERE id IN ($2)`)).
		WithArgs("now()", rowIdStr).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// insert the new version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z"}`), payload_parser.WithAllowPatch())
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting object")
	asserts.Equal(1, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObject_without_previous_version_and_partial_insert() {
	// "status" is missing in the payload, and it can not be read from a previous version of the object
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is a previous version for this object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2)`)).
		WithArgs("Infinity", "1").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}))
	// insert the new version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	_, err := uc.IngestObject(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z"}`), payload_parser.WithAllowPatch())
	asserts := assert.New(t)
	asserts.ErrorAs(err, &models.IngestionValidationErrors{}, "Error ingesting object")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObjects_nominal() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is no previous version for these objects
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2,$3)`)).
		WithArgs("Infinity", "1", "2").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}))
	// insert the new versions
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)`)).
		WithArgs(
			"1", "OK", updAt, 1.0, anyUuid{},
			"2", "OK", updAt, 2.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObjects(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`[{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}, {"object_id": "2", "updated_at": "2020-01-01T00:00:00Z", "value": 2.0, "status": "OK"}]`))
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting objects")
	asserts.Equal(2, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObjects_with_previous_versions() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	rowIdStr1 := "17c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId1 := utils.ByteUuid(rowIdStr1)
	rowIdStr2 := "27c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId2 := utils.ByteUuid(rowIdStr2)
	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there are previous versions for these objects
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2,$3)`)).
		WithArgs("Infinity", "1", "2").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}).
			AddRow("1", "OK", updAt, 1.0, rowId1).
			AddRow("2", "OK", updAt, 2.0, rowId2))
	// update the previous versions
	suite.executorFactory.Mock.ExpectExec(escapeSql(`UPDATE "test"."transactions" SET valid_until = $1 WHERE id IN ($2,$3)`)).
		WithArgs("now()", rowIdStr1, rowIdStr2).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))
	// insert the new versions
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)`)).
		WithArgs(
			"1", "OK", updAt, 1.0, anyUuid{},
			"2", "OK", updAt, 2.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObjects(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`[{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}, {"object_id": "2", "updated_at": "2020-01-01T00:00:00Z", "value": 2.0, "status": "OK"}]`))
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting objects")
	asserts.Equal(2, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObjects_with_partial_insert() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	rowIdStr1 := "17c5805e-eb8f-48f1-afd4-10ad5494954b"
	rowId1 := utils.ByteUuid(rowIdStr1)
	updAt, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	// there is a previous version for one object
	suite.executorFactory.Mock.ExpectQuery(escapeSql(`SELECT object_id, status, updated_at, value, id FROM "test"."transactions" WHERE "test"."transactions".valid_until = $1 AND object_id IN ($2,$3)`)).
		WithArgs("Infinity", "1", "2").
		WillReturnRows(pgxmock.NewRows([]string{"object_id", "status", "updated_at", "value", "id"}).
			AddRow("1", "OK", updAt, 1.0, rowId1))
	// update the previous version
	suite.executorFactory.Mock.ExpectExec(escapeSql(`UPDATE "test"."transactions" SET valid_until = $1 WHERE id IN ($2)`)).
		WithArgs("now()", rowIdStr1).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	// insert the new versions
	suite.executorFactory.Mock.ExpectExec(escapeSql(`INSERT INTO "test"."transactions" (object_id,status,updated_at,value,id) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)`)).
		WithArgs("1", "OK", updAt, 1.0, anyUuid{}, "2", "OK", updAt, 2.0, anyUuid{}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))

	suite.dataModelRepository.On("BatchInsertEnumValues", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), models.EnumValues{}, suite.dataModel.Tables["transactions"]).
		Return(nil)

	nb, err := uc.IngestObjects(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`[{"object_id": "1", "updated_at": "2020-01-01T00:00:00Z"}, {"object_id": "2", "updated_at": "2020-01-01T00:00:00Z", "value": 2.0, "status": "OK"}]`), payload_parser.WithAllowPatch())
	asserts := assert.New(t)
	asserts.NoError(err, "Error ingesting objects")
	asserts.Equal(2, nb, "Number of rows affected")
}

func (suite *IngestionUsecaseTestSuite) TestIngestionUsecase_IngestObjects_with_validation_errors() {
	t := suite.T()
	uc := suite.makeUsecase()

	suite.enforceSecurity.On("CanIngest", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("GetDataModel", mock.MatchedBy(matchContext),
		mock.MatchedBy(matchExec), suite.organizationId, false).
		Return(suite.dataModel, nil)

	_, err := uc.IngestObjects(suite.ctx, suite.organizationId, "transactions",
		json.RawMessage(`[{"object_id": "", "updated_at": "2020-01-01T00:00:00Z", "value": 1.0, "status": "OK"}, {"object_id": "2", "updated_at": "2020-01-01T00:00:00Z", "value": 2.0, "status": "OK"}]`))
	asserts := assert.New(t)
	asserts.ErrorAs(err, &models.IngestionValidationErrors{}, "Error ingesting objects")
}

func TestIngestionUsecase(t *testing.T) {
	suite.Run(t, new(IngestionUsecaseTestSuite))
}
