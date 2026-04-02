package usecases

import (
	"context"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DatamodelUsecaseTestSuite struct {
	suite.Suite
	clientDbIndexEditor          *mocks.ClientDbIndexEditor
	enforceSecurity              *mocks.EnforceSecurity
	executorFactory              *mocks.ExecutorFactory
	dataModelRepository          *mocks.DataModelRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	exec                         *mocks.Executor
	transaction                  *mocks.Transaction
	transactionFactory           *mocks.TransactionFactory

	organizationId      uuid.UUID
	dataModel           models.DataModel
	dataModelWithUnique models.DataModel
	uniqueIndexes       []models.UnicityIndex

	repositoryError error
	securityError   error
	ctx             context.Context
}

func (suite *DatamodelUsecaseTestSuite) SetupTest() {
	suite.clientDbIndexEditor = new(mocks.ClientDbIndexEditor)
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.dataModelRepository = new(mocks.DataModelRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}

	suite.organizationId = uuid.MustParse("12345678-1234-5678-9012-345678901234")
	suite.dataModel = models.DataModel{
		Tables: map[string]models.Table{
			"transactions": {
				ID:   "transactions-table-id",
				Name: "transactions",
				Fields: map[string]models.Field{
					"object_id": {
						ID:       "transactions-object-id-field-id",
						TableId:  "transactions-table-id",
						DataType: models.String,
						Name:     "object_id",
						Nullable: false,
					},
					"value": {
						ID:       "transactions-value-field-id",
						TableId:  "transactions-table-id",
						DataType: models.Float,
						Name:     "value",
						Nullable: false,
					},
					"account_id": {
						ID:       "transactions-account-id-field-id",
						TableId:  "transactions-table-id",
						DataType: models.String,
						Name:     "account_id",
						Nullable: false,
					},
					"reference_id": {
						ID:      "transactions-reference-id-field-id",
						TableId: "transactions-table-id",
						DataType: models.String,
						Name:     "reference_id",
					},
					"not_yet_unique_id": {
						ID:      "transactions-not-yet-unique-id-field-id",
						TableId: "transactions-table-id",
						DataType: models.String,
						Name:     "not_yet_unique_id",
					},
					"unique_id": {
						ID:      "transactions-unique-id-field-id",
						TableId: "transactions-table-id",
						DataType: models.String,
						Name:     "unique_id",
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						Name:            "account",
						ParentTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				ID:   "accounts-table-id",
				Name: "accounts",
				Fields: map[string]models.Field{
					"object_id": {
						ID:       "accounts-object-id-field-id",
						TableId:  "accounts-table-id",
						DataType: models.String,
						Name:     "object_id",
						Nullable: false,
					},
					"status": {
						ID:       "accounts-status-field-id",
						TableId:  "accounts-table-id",
						DataType: models.String,
						Name:     "status",
						Nullable: false,
						IsEnum:   true,
					},
					"balance": {
						ID:       "accounts-balance-field-id",
						TableId:  "accounts-table-id",
						DataType: models.Int,
						Name:     "balance",
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{},
			},
		},
	}
	suite.dataModelWithUnique = models.DataModel{
		Tables: map[string]models.Table{
			"transactions": {
				ID:   "transactions-table-id",
				Name: "transactions",
				Fields: map[string]models.Field{
					"object_id": {
						ID:                "transactions-object-id-field-id",
						TableId:           "transactions-table-id",
						DataType:          models.String,
						Name:              "object_id",
						Nullable:          false,
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
					"value": {
						ID:       "transactions-value-field-id",
						TableId:  "transactions-table-id",
						DataType: models.Float,
						Name:     "value",
						Nullable: false,
					},
					"account_id": {
						ID:       "transactions-account-id-field-id",
						TableId:  "transactions-table-id",
						DataType: models.String,
						Name:     "account_id",
						Nullable: false,
					},
					"reference_id": {
						ID:                "transactions-reference-id-field-id",
						TableId:           "transactions-table-id",
						DataType:          models.String,
						Name:              "reference_id",
						UnicityConstraint: models.PendingUniqueConstraint,
					},
					"not_yet_unique_id": {
						ID:      "transactions-not-yet-unique-id-field-id",
						TableId: "transactions-table-id",
						DataType: models.String,
						Name:     "not_yet_unique_id",
					},
					"unique_id": {
						ID:                "transactions-unique-id-field-id",
						TableId:           "transactions-table-id",
						DataType:          models.String,
						Name:              "unique_id",
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						Name:            "account",
						ParentTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				ID:   "accounts-table-id",
				Name: "accounts",
				Fields: map[string]models.Field{
					"object_id": {
						ID:                "accounts-object-id-field-id",
						TableId:           "accounts-table-id",
						DataType:          models.String,
						Name:              "object_id",
						Nullable:          false,
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
					"status": {
						ID:       "accounts-status-field-id",
						TableId:  "accounts-table-id",
						DataType: models.String,
						Name:     "status",
						Nullable: false,
						IsEnum:   true,
					},
					"balance": {
						ID:       "accounts-balance-field-id",
						TableId:  "accounts-table-id",
						DataType: models.Int,
						Name:     "balance",
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{},
			},
		},
	}
	suite.uniqueIndexes = []models.UnicityIndex{
		{
			TableName: "transactions",
			Fields:    []string{"object_id"},
		},
		{
			TableName:         "transactions",
			Fields:            []string{"reference_id"},
			CreationInProcess: true,
		},
		{
			TableName: "transactions",
			Fields:    []string{"unique_id"},
		},
		{
			TableName: "accounts",
			Fields:    []string{"object_id"},
		},
	}

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
}

func (suite *DatamodelUsecaseTestSuite) makeUsecase() *usecase {
	return &usecase{
		clientDbIndexEditor:          suite.clientDbIndexEditor,
		dataModelRepository:          suite.dataModelRepository,
		enforceSecurity:              suite.enforceSecurity,
		executorFactory:              suite.executorFactory,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		transactionFactory:           suite.transactionFactory,
	}
}

func (suite *DatamodelUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.dataModelRepository.AssertExpectations(t)
	suite.organizationSchemaRepository.AssertExpectations(t)
	suite.exec.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
}

// GetDataModel
func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_nominal_no_unique() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	var nilStr *string
	suite.dataModelRepository.On("ListPivots", suite.ctx, suite.transaction,
		suite.organizationId, nilStr, mock.Anything).
		Return(nil, nil)
	suite.clientDbIndexEditor.On("ListAllIndexes", suite.ctx, suite.organizationId, models.IndexTypeNavigation).
		Return(nil, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return([]models.UnicityIndex{}, nil)

	dataModel, err := usecase.GetDataModel(suite.ctx, suite.organizationId, models.DataModelReadOptions{
		IncludeEnums:              true,
		IncludeNavigationOptions:  true,
		IncludeUnicityConstraints: true,
	}, false)
	suite.Require().NoError(err, "no error expected")
	suite.Require().Equal(suite.dataModel, dataModel, "suite data model should be returned, without changes")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_nominal_with_unique() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)

	var nilStr *string
	suite.dataModelRepository.On("ListPivots", suite.ctx, suite.transaction,
		suite.organizationId, nilStr, mock.Anything).
		Return(nil, nil)
	suite.clientDbIndexEditor.On("ListAllIndexes", suite.ctx, suite.organizationId, models.IndexTypeNavigation).
		Return(nil, nil)
	dataModel, err := usecase.GetDataModel(suite.ctx, suite.organizationId, models.DataModelReadOptions{
		IncludeEnums:              true,
		IncludeNavigationOptions:  true,
		IncludeUnicityConstraints: true,
	}, false)
	suite.Require().NoError(err, "no error expected")
	suite.Require().Equal(suite.dataModelWithUnique, dataModel,
		"suite data model with unicity status should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_security_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(suite.securityError)

	_, err := usecase.GetDataModel(suite.ctx, suite.organizationId, models.DataModelReadOptions{
		IncludeEnums:              true,
		IncludeNavigationOptions:  true,
		IncludeUnicityConstraints: true,
	}, false)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_repository_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true, mock.Anything).
		Return(models.DataModel{}, suite.repositoryError)

	_, err := usecase.GetDataModel(suite.ctx, suite.organizationId, models.DataModelReadOptions{
		IncludeEnums:              true,
		IncludeNavigationOptions:  true,
		IncludeUnicityConstraints: true,
	}, false)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// CreateDataModelTable
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_nominal() {
	usecase := suite.makeUsecase()
	tableName := "name"
	input := models.CreateTableInput{
		Name:         tableName,
		Description:  "description",
		SemanticType: models.SemanticTypeOther,
	}
	// DataModel with required fields for SemanticTypeOther validation
	nameTableDataModel := models.DataModel{
		Tables: map[string]models.Table{
			"name": {
				Name: "name",
				Fields: map[string]models.Field{
					"object_id":  {DataType: models.String, Nullable: false},
					"updated_at": {DataType: models.Timestamp, Nullable: false},
				},
				LinksToSingle: map[string]models.LinkToSingle{},
			},
		},
	}
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// Pre-transaction GetDataModel call (line 242): register Once() first (FIFO, used first, then exhausted).
	// Then unlimited nameTableDataModel for validateTableSemanticType inside the transaction.
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	// Pre-transaction call returns empty DataModel (no links to resolve) - used first via FIFO
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Once().Return(models.DataModel{}, nil)
	// validateTableSemanticType call (SemanticTypeOther requires object_id + updated_at) - unlimited fallback
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(nameTableDataModel, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"),
		mock.AnythingOfType("models.CreateTableInput")).
		Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, mock.AnythingOfType("string")).
		Return(models.TableMetadata{Name: tableName, OrganizationID: suite.organizationId}, nil)
	// ensureTableHasPivot: return non-empty pivots so no pivot creation needed
	suite.dataModelRepository.On("ListPivots", suite.ctx, suite.transaction, suite.organizationId,
		mock.AnythingOfType("*string"), false).
		Return([]models.PivotMetadata{{Id: uuid.New()}}, nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, suite.transaction).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateTable", suite.ctx, suite.transaction, tableName).
		Return(nil)
	suite.clientDbIndexEditor.On("CreateUniqueIndex",
		suite.ctx,
		suite.transaction,
		suite.organizationId,
		models.UnicityIndex{
			TableName: tableName,
			Fields:    []string{"object_id"},
			Included:  []string{"updated_at", "id"},
		}).
		Return(nil)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_repository_error() {
	usecase := suite.makeUsecase()
	tableName := "name"
	input := models.CreateTableInput{
		Name:         tableName,
		Description:  "description",
		SemanticType: models.SemanticTypeOther,
	}
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// Pre-transaction GetDataModel call (line 242)
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{}, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"),
		mock.AnythingOfType("models.CreateTableInput")).
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_org_repository_error() {
	usecase := suite.makeUsecase()
	tableName := "name"
	input := models.CreateTableInput{
		Name:         tableName,
		Description:  "description",
		SemanticType: models.SemanticTypeOther,
	}
	// DataModel with required fields for SemanticTypeOther validation
	nameTableDataModel := models.DataModel{
		Tables: map[string]models.Table{
			"name": {
				Name: "name",
				Fields: map[string]models.Field{
					"object_id":  {DataType: models.String, Nullable: false},
					"updated_at": {DataType: models.Timestamp, Nullable: false},
				},
				LinksToSingle: map[string]models.LinkToSingle{},
			},
		},
	}
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// Pre-transaction GetDataModel call (line 242): register Once() first (FIFO, used first, then exhausted).
	// Then unlimited nameTableDataModel for validateTableSemanticType inside the transaction.
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	// Pre-transaction call returns empty DataModel (no links to resolve) - used first via FIFO
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Once().Return(models.DataModel{}, nil)
	// validateTableSemanticType call (SemanticTypeOther requires object_id + updated_at) - unlimited fallback
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(nameTableDataModel, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"),
		mock.AnythingOfType("models.CreateTableInput")).
		Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, mock.AnythingOfType("string")).
		Return(models.TableMetadata{Name: tableName, OrganizationID: suite.organizationId}, nil)
	// ensureTableHasPivot: return non-empty pivots so no pivot creation needed
	suite.dataModelRepository.On("ListPivots", suite.ctx, suite.transaction, suite.organizationId,
		mock.AnythingOfType("*string"), false).
		Return([]models.PivotMetadata{{Id: uuid.New()}}, nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, suite.transaction).
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_security_error() {
	usecase := suite.makeUsecase()
	input := models.CreateTableInput{
		Name:         "name",
		Description:  "description",
		SemanticType: models.SemanticTypeOther,
	}
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_tableNameTooLong() {
	usecase := suite.makeUsecase()
	name := strings.Repeat("a", 64)
	input := models.CreateTableInput{
		Name:         name,
		Description:  "description",
		SemanticType: models.SemanticTypeOther,
	}
	// WriteDataModel may still be called before validation, but we don't care if validation fails first.
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, input)
	suite.Require().Error(err, "error expected")
	suite.Assert().ErrorIs(err, models.BadParameterError)

	suite.AssertExpectations()
}

// UpdateDataModelTable
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelTable_nominal() {
	tableId := "tableId"
	table := models.TableMetadata{
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelTable",
		suite.ctx, suite.transaction, tableId, utils.Ptr("description"),
		pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId,
		utils.Ptr("description"),
		pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelTable_security_error() {
	tableId := "tableId"
	table := models.TableMetadata{
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId, utils.Ptr("description"),
		pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelTable_repository_error() {
	tableId := "tableId"
	table := models.TableMetadata{
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelTable",
		suite.ctx, suite.transaction, tableId, utils.Ptr("description"),
		pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	).
		Return(suite.repositoryError)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId,
		utils.Ptr("description"),
		pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelTable_nominal_set_ftm_entity() {
	tableId := "tableId"
	table := models.TableMetadata{
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
		FTMEntity:      nil,
	}
	ftmEntity := models.FollowTheMoneyEntityPerson
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelTable",
		suite.ctx, suite.transaction, tableId, utils.Ptr("description"),
		pure_utils.NullFrom(ftmEntity),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId, utils.Ptr("description"),
		pure_utils.NullFrom(ftmEntity),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[models.SemanticType](nil),
		pure_utils.NullFromPtr[string](nil),
		pure_utils.NullFromPtr[string](nil),
	)
	suite.Require().NoError(err, "no error expected when setting FTM entity on table without one")

	suite.AssertExpectations()
}

// CreateDataModelField
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_nominal_not_unique() {
	tableId := "tableId"
	field := models.CreateFieldInput{
		Name:     "name",
		DataType: models.String,
		Nullable: false,
		TableId:  tableId,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), field).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("CreateField", suite.ctx, suite.transaction, table.Name, field).
		Return(nil)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_nominal_unique() {
	tableId := "tableId"
	field := models.CreateFieldInput{
		Name:     "name",
		DataType: models.String,
		Nullable: false,
		IsUnique: true,
		TableId:  tableId,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), field).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("CreateField", suite.ctx, suite.transaction, table.Name, field).
		Return(nil)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_security_error() {
	tableId := "tableId"
	field := models.CreateFieldInput{
		Name:     "name",
		DataType: models.String,
		Nullable: false,
		TableId:  tableId,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_repository_error() {
	tableId := "tableId"
	field := models.CreateFieldInput{
		Name:     "name",
		DataType: models.String,
		Nullable: false,
		TableId:  tableId,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), field).
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_client_schema_repository_error() {
	tableId := "tableId"
	field := models.CreateFieldInput{
		Name:     "name",
		DataType: models.String,
		Nullable: false,
		TableId:  tableId,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), field).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("CreateField", suite.ctx, suite.transaction, table.Name, field).
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// DeleteDataModel
func (suite *DatamodelUsecaseTestSuite) TestDeleteDataModel_nominal() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("DeleteDataModel", suite.ctx, suite.transaction, suite.organizationId).
		Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("DeleteSchema", suite.ctx, suite.transaction).Return(nil)
	err := usecase.DeleteDataModel(suite.ctx, suite.organizationId)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestDeleteDataModel_security_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := usecase.DeleteDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestDeleteDataModel_repository_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("DeleteDataModel", suite.ctx, suite.transaction, suite.organizationId).
		Return(suite.repositoryError)

	err := usecase.DeleteDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestDeleteDataModel_client_schema_repository_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("DeleteDataModel", suite.ctx, suite.transaction, suite.organizationId).
		Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("DeleteSchema", suite.ctx, suite.transaction).Return(suite.repositoryError)

	err := usecase.DeleteDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// CreateDataModelLink
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_nominal() {
	link := models.DataModelLinkCreateInput{
		OrganizationID: suite.organizationId,
		Name:           "name",
		LinkType:       models.LinkTypeRelated,
		ParentTableID:  "accounts-table-id",
		ChildTableID:   "transactions-table-id",
		ParentFieldID:  "accounts-object-id-field-id",
		ChildFieldID:   "transactions-account-id-field-id",
	}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	// getDataModelWithExec inside createDataModelLinkWithExec, and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.dataModelRepository.On("CreateDataModelLink", suite.ctx, suite.transaction, mock.AnythingOfType("string"), link).
		Return(nil)

	_, err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_parent_field_not_object_id() {
	link := models.DataModelLinkCreateInput{
		OrganizationID: suite.organizationId,
		Name:           "name",
		LinkType:       models.LinkTypeRelated,
		ParentTableID:  "accounts-table-id",
		ChildTableID:   "transactions-table-id",
		ParentFieldID:  "accounts-balance-field-id", // balance, not object_id
		ChildFieldID:   "transactions-account-id-field-id",
	}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	// getDataModelWithExec inside createDataModelLinkWithExec
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)

	_, err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().ErrorContains(err, "parent field must be the object_id field", "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_child_field_not_string() {
	link := models.DataModelLinkCreateInput{
		OrganizationID: suite.organizationId,
		Name:           "name",
		LinkType:       models.LinkTypeRelated,
		ParentTableID:  "accounts-table-id",
		ChildTableID:   "transactions-table-id",
		ParentFieldID:  "accounts-object-id-field-id",
		ChildFieldID:   "transactions-value-field-id", // value is Float, not String
	}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	// getDataModelWithExec inside createDataModelLinkWithExec
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)

	_, err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().ErrorContains(err, "child field must be a string", "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_security_error() {
	link := models.DataModelLinkCreateInput{OrganizationID: suite.organizationId}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	_, err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_repository_error() {
	link := models.DataModelLinkCreateInput{OrganizationID: suite.organizationId, Name: "name", LinkType: models.LinkTypeRelated}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	// getDataModelWithExec inside createDataModelLinkWithExec fails
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(models.DataModel{}, suite.repositoryError)

	_, err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// UpdateDataModelField
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_nominal_update_desc() {
	fieldId := "fieldId"
	tableId := "tableId"
	newDesc := "new description"
	input := models.UpdateFieldInput{Description: &newDesc}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "value", DataType: models.Float, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_nominal_update_enum() {
	fieldId := "fieldId"
	tableId := "tableId"
	newIsEnum := true
	input := models.UpdateFieldInput{IsEnum: &newIsEnum}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "value", DataType: models.Float, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_nominal_update_unique() {
	fieldId := "fieldId"
	tableId := "tableId"

	newIsUnique := true
	input := models.UpdateFieldInput{IsUnique: &newIsUnique}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{
			Name:     "not_yet_unique_id",
			DataType: models.Float,
			ID:       fieldId,
			IsEnum:   false,
			TableId:  tableId,
		}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)
	suite.clientDbIndexEditor.On("CreateUniqueIndexAsync", suite.ctx, suite.organizationId, models.UnicityIndex{
		TableName: "transactions",
		Fields:    []string{"not_yet_unique_id"},
	}).Return(nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_nominal_update_not_unique() {
	fieldId := "fieldId"
	tableId := "tableId"
	newIsUnique := false
	input := models.UpdateFieldInput{IsUnique: &newIsUnique}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{
			Name:     "unique_id",
			DataType: models.String,
			ID:       fieldId,
			TableId:  tableId,
		}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)
	suite.clientDbIndexEditor.On("DeleteUniqueIndex", suite.ctx, suite.organizationId, models.UnicityIndex{
		TableName: "transactions",
		Fields:    []string{"unique_id"},
	}).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_security_error() {
	fieldId := "fieldId"
	input := models.UpdateFieldInput{}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, mock.Anything).
		Return(models.TableMetadata{OrganizationID: suite.organizationId}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_repository_error() {
	fieldId := "fieldId"
	input := models.UpdateFieldInput{}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{}, suite.repositoryError)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_with_ftm_property() {
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyName
	ftmEntity := models.FollowTheMoneyEntityPerson
	field := models.CreateFieldInput{
		Name:        "name",
		DataType:    models.String,
		Nullable:    false,
		TableId:     tableId,
		FTMProperty: &ftmProperty,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "name",
		Description:    "description",
		OrganizationID: suite.organizationId,
		FTMEntity:      &ftmEntity,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), field).
		Return(nil)
	// validateTableSemanticType: table "name" has SemanticTypeUnset → noOpValidation
	suite.dataModelRepository.On("GetDataModel", suite.ctx, suite.transaction, suite.organizationId, false, false).
		Return(models.DataModel{Tables: map[string]models.Table{"name": {Name: "name"}}}, nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.organizationSchemaRepository.On("CreateField", suite.ctx, suite.transaction, table.Name, field).
		Return(nil)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

// UpdateDataModelField with FTM Property
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_with_ftm_property() {
	fieldId := "fieldId"
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyEmail
	ftmEntity := models.FollowTheMoneyEntityPerson
	input := models.UpdateFieldInput{
		FTMProperty: pure_utils.NullFrom(ftmProperty),
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "email", DataType: models.String, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "customers",
			OrganizationID: suite.organizationId,
			FTMEntity:      &ftmEntity,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	dataModelWithFTM := suite.dataModel
	dataModelWithFTM.Tables["customers"] = models.Table{
		Name:      "customers",
		FTMEntity: &ftmEntity,
	}
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(dataModelWithFTM, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

// UpdateDataModelField to clear FTM Property
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_clear_ftm_property() {
	fieldId := "fieldId"
	tableId := "tableId"
	input := models.UpdateFieldInput{
		FTMProperty: pure_utils.NullFromPtr[models.FollowTheMoneyProperty](nil),
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{
			Name:        "email",
			DataType:    models.String,
			ID:          fieldId,
			IsEnum:      false,
			TableId:     tableId,
			FTMProperty: utils.Ptr(models.FollowTheMoneyPropertyEmail),
		}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "customers",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// DataModel must include "customers" table for validateTableSemanticType
	dataModelWithCustomers := suite.dataModel
	dataModelWithCustomers.Tables["customers"] = models.Table{Name: "customers"}
	// getDataModelWithExec (IncludeUnicityConstraints: true) and validateTableSemanticType both call GetDataModel
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(dataModelWithCustomers, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

// CreateDataModelField with invalid FTM property for entity
// RegistrationNumber is invalid for Person entity (only valid for Company, Organization, Vessel)
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelField_with_invalid_ftm_property_for_entity() {
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyRegistrationNumber
	ftmEntity := models.FollowTheMoneyEntityPerson
	field := models.CreateFieldInput{
		Name:        "registration_num",
		DataType:    models.String,
		Nullable:    false,
		TableId:     tableId,
		FTMProperty: &ftmProperty,
	}
	table := models.TableMetadata{
		ID:             tableId,
		Name:           "people",
		Description:    "description",
		OrganizationID: suite.organizationId,
		FTMEntity:      &ftmEntity,
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)

	_, err := usecase.CreateDataModelField(suite.ctx, field)
	suite.Require().Error(err, "error expected: RegistrationNumber is not valid for Person entity")
	suite.Require().ErrorContains(err, "invalid FTM property for entity",
		"expected error message about invalid property")

	suite.AssertExpectations()
}

// UpdateDataModelField with invalid FTM property for entity
// CageCode is invalid for Person entity (only valid for Company)
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_with_invalid_ftm_property_for_entity() {
	fieldId := "fieldId"
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyRegistrationNumber
	ftmEntity := models.FollowTheMoneyEntityPerson
	input := models.UpdateFieldInput{
		FTMProperty: pure_utils.NullFrom(ftmProperty),
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "cage_code", DataType: models.String, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "people",
			OrganizationID: suite.organizationId,
			FTMEntity:      &ftmEntity,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true)
	dataModelWithPeople := suite.dataModel
	dataModelWithPeople.Tables["people"] = models.Table{
		Name:      "people",
		FTMEntity: &ftmEntity,
	}
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(dataModelWithPeople, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected: CageCode is not valid for Person entity")
	suite.Require().ErrorContains(err, "invalid FTM property for entity",
		"expected error message about invalid property")

	suite.AssertExpectations()
}

// UpdateDataModelField with FTM property but no entity defined on table
// Cannot set FTM property without table FTM entity
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_with_ftm_property_but_no_entity() {
	fieldId := "fieldId"
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyEmail
	input := models.UpdateFieldInput{
		FTMProperty: pure_utils.NullFrom(ftmProperty),
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "email", DataType: models.String, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	// Table has NO FTM entity defined
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "customers",
			OrganizationID: suite.organizationId,
			FTMEntity:      nil,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected: cannot set FTM property without table FTM entity")
	suite.Require().ErrorContains(err, "FTM entity not defined for table",
		"expected error message about missing entity")

	suite.AssertExpectations()
}

// UpdateDataModelField with PassportNumber property on Organization entity (invalid)
// PassportNumber is only valid for Person entity
func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_with_invalid_passport_on_organization() {
	fieldId := "fieldId"
	tableId := "tableId"
	ftmProperty := models.FollowTheMoneyPropertyPassportNumber
	ftmEntity := models.FollowTheMoneyEntityOrganization
	input := models.UpdateFieldInput{
		FTMProperty: pure_utils.NullFrom(ftmProperty),
	}
	usecase := suite.makeUsecase()
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "passport_num", DataType: models.String, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "organizations",
			OrganizationID: suite.organizationId,
			FTMEntity:      &ftmEntity,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// getDataModelWithExec (IncludeUnicityConstraints: true)
	dataModelWithOrganizations := suite.dataModel
	dataModelWithOrganizations.Tables["organizations"] = models.Table{
		Name:          "organizations",
		Fields:        map[string]models.Field{},
		LinksToSingle: make(map[string]models.LinkToSingle),
		FTMEntity:     &ftmEntity,
	}
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, false, mock.Anything).
		Return(dataModelWithOrganizations, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx, suite.organizationId).
		Return(suite.uniqueIndexes, nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected: PassportNumber is not valid for Organization entity")
	suite.Require().ErrorContains(err, "invalid FTM property for entity",
		"expected error message about invalid property")

	suite.AssertExpectations()
}

func TestDatamodelUsecase(t *testing.T) {
	suite.Run(t, new(DatamodelUsecaseTestSuite))
}
