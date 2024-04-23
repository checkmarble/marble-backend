package utils

import "github.com/checkmarble/marble-backend/models"

const (
	DummyTableNameFirst  = "first"
	DummyTableNameSecond = "second"
	DummyTableNameThird  = "third"
)

const (
	DummyFieldNameId           = "id"
	DummyFieldNameForBool      = "bool_var"
	DummyFieldNameForInt       = "int_var"
	DummyFieldNameForFloat     = "float_var"
	DummyFieldNameForTimestamp = "time_var"
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

	dummyFirstLinkToSingle := map[string]models.LinkToSingle{
		DummyTableNameSecond: {
			ParentTableName: DummyTableNameSecond,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	dummySecondLinkToSingle := map[string]models.LinkToSingle{
		DummyTableNameThird: {
			ParentTableName: DummyTableNameThird,
			ParentFieldName: DummyFieldNameId,
			ChildFieldName:  DummyFieldNameId,
		},
	}

	dummyFieldsIdOnly := map[string]models.Field{
		DummyFieldNameId: dummyFieldString,
	}

	dummyAllFields := map[string]models.Field{
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
		Tables: map[string]models.Table{
			DummyTableNameFirst:  dummyDataModelFirstTable,
			DummyTableNameSecond: dummyDataModelSecondTable,
			DummyTableNameThird:  dummyDataModelThirdTable,
		},
	}
}
