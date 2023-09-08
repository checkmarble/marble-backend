package evaluate_test

import (
	"marble/marble-backend/models"
)

//
// For Custom List Evaluator
//
const testListId string = "1"
const testListOrgId string = "2"

var testList models.CustomList = models.CustomList{
	Id:             testListId,
	OrganizationId: testListOrgId,
}

var testCustomListNamedArgs = map[string]any{
	"customListId": testListId,
}

//
// For Database Access Evaluator
//
const testTableNameFirst models.TableName = "first"
const testTableNameSecond models.TableName = "second"

const testFieldNameId models.FieldName = "id"
const testFieldNameForBool models.FieldName = "bool_var"
const testFieldNameForInt models.FieldName = "int_var"
const testFieldNameForFloat models.FieldName = "float_var"
const testFieldNameForTimestamp models.FieldName = "time_var"

func getTestFirstDataModel() models.DataModel {
	var testFieldBool models.Field = models.Field{
		DataType: 0,
		Nullable: false,
	}
	var testFieldInt models.Field = models.Field{
		DataType: 1,
		Nullable: false,
	}
	var testFieldFloat models.Field = models.Field{
		DataType: 2,
		Nullable: false,
	}
	
	var testFieldString models.Field = models.Field{
		DataType: 3,
		Nullable: false,
	}
		
	var testFieldTimestamp models.Field = models.Field{
		DataType: 4,
		Nullable: false,
	}

	var testFirstLinkToSingle map[models.LinkName]models.LinkToSingle = map[models.LinkName]models.LinkToSingle{
		models.LinkName(testTableNameSecond): {
			LinkedTableName: testTableNameSecond,
			ParentFieldName: testFieldNameId,
			ChildFieldName:  testFieldNameId,
		},
	}
	
	var testFieldsFirst map[models.FieldName]models.Field = map[models.FieldName]models.Field{
		testFieldNameId: testFieldString,
	}
	
	var testFieldsSecond map[models.FieldName]models.Field = map[models.FieldName]models.Field{
		testFieldNameId: testFieldString,
		testFieldNameForInt: testFieldInt,
		testFieldNameForFloat: testFieldFloat,
		testFieldNameForBool: testFieldBool,
		testFieldNameForTimestamp: testFieldTimestamp, 
	}
	
	var testDataModelFirstTable models.Table = models.Table{
		Name:          testTableNameFirst,
		Fields:        testFieldsFirst,
		LinksToSingle: testFirstLinkToSingle,
	}
	
	var testDataModelSecondTable models.Table = models.Table{
		Name:          testTableNameSecond,
		Fields:        testFieldsSecond,
	}

	return models.DataModel{
		Version: "1",
		Status:  0,
		Tables:  map[models.TableName]models.Table{
			testTableNameFirst: testDataModelFirstTable,
			testTableNameSecond: testDataModelSecondTable,
		},
	}
}