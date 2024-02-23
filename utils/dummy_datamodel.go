package utils

import "github.com/checkmarble/marble-backend/models"

const (
	DummyTableNameFirst  models.TableName = "first"
	DummyTableNameSecond models.TableName = "second"
	DummyTableNameThird  models.TableName = "third"
)

const (
	DummyFieldNameId           models.FieldName = "id"
	DummyFieldNameForBool      models.FieldName = "bool_var"
	DummyFieldNameForInt       models.FieldName = "int_var"
	DummyFieldNameForFloat     models.FieldName = "float_var"
	DummyFieldNameForTimestamp models.FieldName = "time_var"
)

func GetDummyDataModel() models.DataModel {
	dummyFieldBool := models.Field{
		DataType: 0,
		Nullable: false,
	}
	dummyFieldInt := models.Field{
		DataType: 1,
		Nullable: false,
	}
	dummyFieldFloat := models.Field{
		DataType: 2,
		Nullable: false,
	}

	dummyFieldString := models.Field{
		DataType: 3,
		Nullable: false,
	}

	dummyFieldTimestamp := models.Field{
		DataType: 4,
		Nullable: false,
	}

	dummyFirstLinkToSingle := map[models.LinkName]models.LinkToSingle{
		models.LinkName(DummyTableNameSecond): {
			LinkedTableName: DummyTableNameSecond,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	dummySecondLinkToSingle := map[models.LinkName]models.LinkToSingle{
		models.LinkName(DummyTableNameThird): {
			LinkedTableName: DummyTableNameThird,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	dummyFieldsIdOnly := map[models.FieldName]models.Field{
		DummyFieldNameId: dummyFieldString,
	}

	dummyAllFields := map[models.FieldName]models.Field{
		DummyFieldNameId:           dummyFieldString,
		DummyFieldNameForInt:       dummyFieldInt,
		DummyFieldNameForFloat:     dummyFieldFloat,
		DummyFieldNameForBool:      dummyFieldBool,
		DummyFieldNameForTimestamp: dummyFieldTimestamp,
	}

	dummyDataModelFirstTable := models.Table{
		Name:          DummyTableNameFirst,
		Fields:        dummyFieldsIdOnly,
		LinksToSingle: dummyFirstLinkToSingle,
	}

	dummyDataModelSecondTable := models.Table{
		Name:          DummyTableNameSecond,
		Fields:        dummyAllFields,
		LinksToSingle: dummySecondLinkToSingle,
	}

	dummyDataModelThirdTable := models.Table{
		Name:   DummyTableNameThird,
		Fields: dummyAllFields,
	}

	return models.DataModel{
		Version: "1",
		Tables: map[models.TableName]models.Table{
			DummyTableNameFirst:  dummyDataModelFirstTable,
			DummyTableNameSecond: dummyDataModelSecondTable,
			DummyTableNameThird:  dummyDataModelThirdTable,
		},
	}
}
