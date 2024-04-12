package usecases

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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
	transaction                  *mocks.Executor
	transactionFactory           *mocks.TransactionFactory

	organizationId      string
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
	suite.transaction = new(mocks.Executor)
	suite.transactionFactory = &mocks.TransactionFactory{ExecMock: suite.transaction}

	suite.organizationId = "organizationId"
	suite.dataModel = models.DataModel{
		Tables: map[string]models.Table{
			"transactions": {
				Name: "transactions",
				Fields: map[string]models.Field{
					"object_id": {
						DataType: models.String,
						Name:     "object_id",
						Nullable: false,
					},
					"value": {
						DataType: models.Float,
						Name:     "value",
						Nullable: false,
					},
					"account_id": {
						DataType: models.String,
						Name:     "account_id",
						Nullable: false,
					},
					"reference_id": {
						DataType: models.String,
						Name:     "reference_id",
					},
					"not_yet_unique_id": {
						DataType: models.String,
						Name:     "not_yet_unique_id",
					},
					"unique_id": {
						DataType: models.String,
						Name:     "unique_id",
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						Name:            "account",
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]models.Field{
					"object_id": {
						DataType: models.String,
						Name:     "object_id",
						Nullable: false,
					},
					"status": {
						DataType: models.String,
						Name:     "status",
						Nullable: false,
						IsEnum:   true,
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{},
			},
		},
	}
	suite.dataModelWithUnique = models.DataModel{
		Tables: map[string]models.Table{
			"transactions": {
				Name: "transactions",
				Fields: map[string]models.Field{
					"object_id": {
						DataType:          models.String,
						Name:              "object_id",
						Nullable:          false,
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
					"value": {
						DataType: models.Float,
						Name:     "value",
						Nullable: false,
					},
					"account_id": {
						DataType: models.String,
						Name:     "account_id",
						Nullable: false,
					},
					"reference_id": {
						DataType:          models.String,
						Name:              "reference_id",
						UnicityConstraint: models.PendingUniqueConstraint,
					},
					"not_yet_unique_id": {
						DataType: models.String,
						Name:     "not_yet_unique_id",
					},
					"unique_id": {
						DataType:          models.String,
						Name:              "unique_id",
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						Name:            "account",
						LinkedTableName: "accounts",
						ParentFieldName: "object_id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]models.Field{
					"object_id": {
						DataType:          models.String,
						Name:              "object_id",
						Nullable:          false,
						UnicityConstraint: models.ActiveUniqueConstraint,
					},
					"status": {
						DataType: models.String,
						Name:     "status",
						Nullable: false,
						IsEnum:   true,
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

func (suite *DatamodelUsecaseTestSuite) makeUsecase() *DataModelUseCase {
	return &DataModelUseCase{
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
	suite.transaction.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
}

// GetDataModel
func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_nominal_no_unique() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return([]models.UnicityIndex{}, nil)

	dataModel, err := usecase.GetDataModel(suite.ctx, suite.organizationId)
	suite.Require().NoError(err, "no error expected")
	suite.Require().Equal(suite.dataModel, dataModel, "suite data model should be returned, without changes")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_nominal_with_unique() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return(suite.uniqueIndexes, nil)

	dataModel, err := usecase.GetDataModel(suite.ctx, suite.organizationId)
	suite.Require().NoError(err, "no error expected")
	suite.Require().Equal(suite.dataModelWithUnique, dataModel,
		"suite data model with unicity status should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_security_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(suite.securityError)

	_, err := usecase.GetDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestGetDataModel_repository_error() {
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(models.DataModel{}, suite.repositoryError)

	_, err := usecase.GetDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// CreateDataModelTable
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_nominal() {
	usecase := suite.makeUsecase()
	tableName := "name"
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), "name", "description").
		Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx,
		suite.transaction,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.CreateFieldInput")).
		Return(nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, suite.transaction).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateTable", suite.ctx, suite.transaction, tableName).
		Return(nil)
	suite.clientDbIndexEditor.On("CreateUniqueIndex",
		suite.ctx, suite.transaction, models.UnicityIndex{
			TableName: tableName,
			Fields:    []string{"object_id"},
			Included:  []string{"updated_at", "id"},
		}).
		Return(nil)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, tableName, "description")
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_repository_error() {
	usecase := suite.makeUsecase()
	tableName := "name"
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), "name", "description").
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, tableName, "description")
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_org_repository_error() {
	usecase := suite.makeUsecase()
	tableName := "name"
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.dataModelRepository.On("CreateDataModelTable",
		suite.ctx, suite.transaction, suite.organizationId, mock.AnythingOfType("string"), "name", "description").
		Return(nil)
	suite.dataModelRepository.On("CreateDataModelField",
		suite.ctx,
		suite.transaction,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("models.CreateFieldInput")).
		Return(nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).
		Return(nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, suite.transaction).
		Return(suite.repositoryError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, tableName, "description")
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelTable_security_error() {
	usecase := suite.makeUsecase()
	tableName := "name"
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	_, err := usecase.CreateDataModelTable(suite.ctx, suite.organizationId, tableName, "description")
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelTable",
		suite.ctx, suite.transaction, tableId, "description").
		Return(nil)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId, "description")
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId, "description")
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(table, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.dataModelRepository.On("UpdateDataModelTable",
		suite.ctx, suite.transaction, tableId, "description").
		Return(suite.repositoryError)

	err := usecase.UpdateDataModelTable(suite.ctx, tableId, "description")
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

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
		suite.ctx, suite.transaction, mock.AnythingOfType("string"), field).
		Return(nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).Return(nil)
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
		suite.ctx, suite.transaction, mock.AnythingOfType("string"), field).
		Return(nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).Return(nil)
	suite.organizationSchemaRepository.On("CreateField", suite.ctx, suite.transaction, table.Name, field).
		Return(nil)
	suite.clientDbIndexEditor.On("CreateUniqueIndexAsync", suite.ctx, models.UnicityIndex{
		TableName: table.Name,
		Fields:    []string{field.Name},
	}).Return(nil)

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
		suite.ctx, suite.transaction, mock.AnythingOfType("string"), field).
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
		suite.ctx, suite.transaction, mock.AnythingOfType("string"), field).
		Return(nil)
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).
		Return(nil)
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
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).Return(nil)
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
	suite.transactionFactory.On("TransactionInOrgSchema", suite.ctx, suite.organizationId, mock.Anything).Return(nil)
	suite.organizationSchemaRepository.On("DeleteSchema", suite.ctx, suite.transaction).Return(suite.repositoryError)

	err := usecase.DeleteDataModel(suite.ctx, suite.organizationId)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

// CreateDataModelLink
func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_nominal() {
	parentTableName := "accounts"
	parentFieldName := "object_id"
	link := models.DataModelLinkCreateInput{
		OrganizationID: suite.organizationId,
		Name:           "name",
		ParentTableID:  "parentTableId",
		ChildTableID:   "childTableId",
		ParentFieldID:  "parentFieldId",
		ChildFieldID:   "childFieldId",
	}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ChildTableID).
		Return(models.TableMetadata{}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ParentTableID).
		Return(models.TableMetadata{Name: parentTableName}, nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, link.ParentFieldID).
		Return(models.FieldMetadata{Name: parentFieldName}, nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, link.ChildFieldID).
		Return(models.FieldMetadata{}, nil)
	suite.dataModelRepository.On("CreateDataModelLink", suite.ctx, suite.transaction, link).
		Return(nil)
	// for GetDataModel (reused in CreateDataModelLink), copied from TestGetDataModel_nominal_with_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return(suite.uniqueIndexes, nil)

	err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_parent_field_not_unique() {
	parentTableName := "accounts"
	parentFieldName := "object_id"
	link := models.DataModelLinkCreateInput{
		OrganizationID: suite.organizationId,
		Name:           "name",
		ParentTableID:  "parentTableId",
		ChildTableID:   "childTableId",
		ParentFieldID:  "parentFieldId",
		ChildFieldID:   "childFieldId",
	}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ChildTableID).
		Return(models.TableMetadata{}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ParentTableID).
		Return(models.TableMetadata{Name: parentTableName}, nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, link.ParentFieldID).
		Return(models.FieldMetadata{Name: parentFieldName}, nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, link.ChildFieldID).
		Return(models.FieldMetadata{}, nil)
	// for GetDataModel (reused in CreateDataModelLink), copied from TestGetDataModel_nominal_no_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return([]models.UnicityIndex{}, nil)

	err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().ErrorContains(err, "parent field must be unique", "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_security_error() {
	link := models.DataModelLinkCreateInput{OrganizationID: suite.organizationId}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := usecase.CreateDataModelLink(suite.ctx, link)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.securityError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestCreateDataModelLink_repository_error() {
	link := models.DataModelLinkCreateInput{OrganizationID: suite.organizationId}
	usecase := suite.makeUsecase()
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ChildTableID).
		Return(models.TableMetadata{}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, link.ParentTableID).
		Return(models.TableMetadata{}, nil)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, link.ChildFieldID).
		Return(models.FieldMetadata{}, suite.repositoryError)

	err := usecase.CreateDataModelLink(suite.ctx, link)
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "value", DataType: models.Float, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// for GetDataModel (reused in UpdateDataModelField), copied from TestGetDataModel_nominal_with_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{Name: "value", DataType: models.Float, ID: fieldId, IsEnum: false, TableId: tableId}, nil)
	suite.dataModelRepository.On("GetDataModelTable", suite.ctx, suite.transaction, tableId).
		Return(models.TableMetadata{
			Name:           "transactions",
			OrganizationID: suite.organizationId,
		}, nil)
	suite.enforceSecurity.On("WriteDataModel", suite.organizationId).Return(nil)
	// for GetDataModel (reused in UpdateDataModelField), copied from TestGetDataModel_nominal_with_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
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
	// for GetDataModel (reused in UpdateDataModelField), copied from TestGetDataModel_nominal_with_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return(suite.uniqueIndexes, nil)
	suite.dataModelRepository.On("UpdateDataModelField", suite.ctx, suite.transaction, fieldId, input).
		Return(nil)
	suite.clientDbIndexEditor.On("CreateUniqueIndexAsync", suite.ctx, models.UnicityIndex{
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
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
	suite.clientDbIndexEditor.On("DeleteUniqueIndex", suite.ctx, models.UnicityIndex{
		TableName: "transactions",
		Fields:    []string{"unique_id"},
	}).Return(nil)
	// for GetDataModel (reused in UpdateDataModelField), copied from TestGetDataModel_nominal_with_unique
	suite.enforceSecurity.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewExecutor").Return(suite.transaction, nil)
	suite.dataModelRepository.On("GetDataModel",
		suite.ctx, suite.transaction, suite.organizationId, true).
		Return(suite.dataModel, nil)
	suite.clientDbIndexEditor.On("ListAllUniqueIndexes", suite.ctx).
		Return(suite.uniqueIndexes, nil)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().NoError(err, "no error expected")

	suite.AssertExpectations()
}

func (suite *DatamodelUsecaseTestSuite) TestUpdateDataModelField_security_error() {
	fieldId := "fieldId"
	input := models.UpdateFieldInput{}
	usecase := suite.makeUsecase()
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
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
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.dataModelRepository.On("GetDataModelField", suite.ctx, suite.transaction, fieldId).
		Return(models.FieldMetadata{}, suite.repositoryError)

	err := usecase.UpdateDataModelField(suite.ctx, fieldId, input)
	suite.Require().Error(err, "error expected")
	suite.Require().Equal(suite.repositoryError, err, "expected error should be returned")

	suite.AssertExpectations()
}

func TestDatamodelUsecase(t *testing.T) {
	suite.Run(t, new(DatamodelUsecaseTestSuite))
}
