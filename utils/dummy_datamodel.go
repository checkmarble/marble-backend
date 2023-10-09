package utils

import "github.com/checkmarble/marble-backend/models"

const DummyTableNameFirst models.TableName = "first"
const DummyTableNameSecond models.TableName = "second"
const DummyTableNameThird models.TableName = "third"

const DummyFieldNameId models.FieldName = "id"
const DummyFieldNameForBool models.FieldName = "bool_var"
const DummyFieldNameForInt models.FieldName = "int_var"
const DummyFieldNameForFloat models.FieldName = "float_var"
const DummyFieldNameForTimestamp models.FieldName = "time_var"

func GetDummyDataModel() models.DataModel {
	var dummyFieldBool = models.Field{
		DataType: 0,
		Nullable: false,
	}
	var dummyFieldInt = models.Field{
		DataType: 1,
		Nullable: false,
	}
	var dummyFieldFloat = models.Field{
		DataType: 2,
		Nullable: false,
	}

	var dummyFieldString = models.Field{
		DataType: 3,
		Nullable: false,
	}

	var dummyFieldTimestamp = models.Field{
		DataType: 4,
		Nullable: false,
	}

	var dummyFirstLinkToSingle = map[models.LinkName]models.LinkToSingle{
		models.LinkName(DummyTableNameSecond): {
			LinkedTableName: DummyTableNameSecond,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	var dummySecondLinkToSingle = map[models.LinkName]models.LinkToSingle{
		models.LinkName(DummyTableNameThird): {
			LinkedTableName: DummyTableNameThird,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	var dummyFieldsIdOnly = map[models.FieldName]models.Field{
		DummyFieldNameId: dummyFieldString,
	}

	var dummyAllFields = map[models.FieldName]models.Field{
		DummyFieldNameId:           dummyFieldString,
		DummyFieldNameForInt:       dummyFieldInt,
		DummyFieldNameForFloat:     dummyFieldFloat,
		DummyFieldNameForBool:      dummyFieldBool,
		DummyFieldNameForTimestamp: dummyFieldTimestamp,
	}

	var dummyDataModelFirstTable = models.Table{
		Name:          DummyTableNameFirst,
		Fields:        dummyFieldsIdOnly,
		LinksToSingle: dummyFirstLinkToSingle,
	}

	var dummyDataModelSecondTable = models.Table{
		Name:          DummyTableNameSecond,
		Fields:        dummyAllFields,
		LinksToSingle: dummySecondLinkToSingle,
	}

	var dummyDataModelThirdTable = models.Table{
		Name:   DummyTableNameThird,
		Fields: dummyAllFields,
	}

	return models.DataModel{
		Version: "1",
		Status:  0,
		Tables: map[models.TableName]models.Table{
			DummyTableNameFirst:  dummyDataModelFirstTable,
			DummyTableNameSecond: dummyDataModelSecondTable,
			DummyTableNameThird:  dummyDataModelThirdTable,
		},
	}
}
