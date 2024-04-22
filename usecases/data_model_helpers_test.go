package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
)

func getTestDataModel() (models.DataModel, models.DataModel) {
	dataModel := models.DataModel{
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
					"timestamp": {
						DataType: models.Timestamp,
						Name:     "timestamp",
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
	dataModelWithUnique := models.DataModel{
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
					"timestamp": {
						DataType: models.Timestamp,
						Name:     "timestamp",
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
	return dataModel, dataModelWithUnique
}

func TestFindLinksToField(t *testing.T) {
	assert := assert.New(t)
	dataModel, _ := getTestDataModel()

	t.Run("with link to parent table accounts object_id", func(t *testing.T) {
		links := findLinksToField(dataModel, "accounts", "object_id")
		assert.Equal([]models.LinkToSingle{{
			Name:            "account",
			ParentTableName: "accounts",
			ParentFieldName: "object_id",
			ChildTableName:  "",
			ChildFieldName:  "account_id",
		}}, links, "expected links to parent table accounts object_id should be returned")
	})

	t.Run("with no link to parent table accounts status", func(t *testing.T) {
		links := findLinksToField(dataModel, "accounts", "status")
		assert.Empty(links, "no links to parent table accounts status should be returned")
	})

	t.Run("with no link to child table transactions object_id", func(t *testing.T) {
		links := findLinksToField(dataModel, "transactions", "object_id")
		assert.Empty(links, "no links to parent table transactions object_id should be returned")
	})
}

func TestValidateFieldUpdateRules(t *testing.T) {
	organizationId := "organizationId"

	dataModel, dataModelWithUnique := getTestDataModel()
	table := models.TableMetadata{
		Name:           "transactions",
		OrganizationID: organizationId,
	}

	t.Run("make unique nominal", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "",
			DataType: models.String,
			IsEnum:   false,
		}
		newTrue := true

		makeUnique, makeNotUnique, err := validateFieldUpdateRules(
			dataModelWithUnique, field, table, models.UpdateFieldInput{
				IsUnique: &newTrue,
			})
		asserts.NoError(err, "no error expected")
		asserts.True(makeUnique, "field should be made unique")
		asserts.False(makeNotUnique, "field should not be made not unique")
	})

	t.Run("make not unique nominal", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "unique_id",
			DataType: models.String,
			IsEnum:   false,
		}
		newFalse := false

		makeUnique, makeNotUnique, err := validateFieldUpdateRules(
			dataModelWithUnique, field, table, models.UpdateFieldInput{
				IsUnique: &newFalse,
			})
		asserts.NoError(err, "no error expected")
		asserts.False(makeUnique, "field should not be made unique")
		asserts.True(makeNotUnique, "field should be made not unique")
	})

	t.Run("make enum nominal", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "status",
			DataType: models.String,
			IsEnum:   false,
		}
		newTrue := true

		makeUnique, makeNotUnique, err := validateFieldUpdateRules(dataModel, field, table, models.UpdateFieldInput{
			IsEnum: &newTrue,
		})
		asserts.NoError(err, "no error expected")
		asserts.False(makeNotUnique, "unicity status should not be changed")
		asserts.False(makeUnique, "unicity status should not be changed")
	})

	t.Run("make enum error", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "unique_id",
			DataType: models.String,
		}
		newTrue := true

		_, _, err := validateFieldUpdateRules(dataModelWithUnique, field, table, models.UpdateFieldInput{
			IsEnum: &newTrue,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "cannot make a field an enum if it is unique", "expected error should be returned")
	})

	t.Run("not enumerable type error", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "timestamp",
			DataType: models.Timestamp,
		}
		newTrue := true

		_, _, err := validateFieldUpdateRules(dataModelWithUnique, field, table, models.UpdateFieldInput{
			IsEnum: &newTrue,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "enum fields can only be of type string or numeric", "expected error should be returned")
	})

	t.Run("type can't be unique error", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "timestamp",
			DataType: models.Timestamp,
		}
		newTrue := true

		_, _, err := validateFieldUpdateRules(dataModel, field, table, models.UpdateFieldInput{
			IsUnique: &newTrue,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "unique fields can only be of type string, int or float", "expected error should be returned")
	})

	t.Run("object_id must always remain unique", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "object_id",
			DataType: models.String,
		}
		newFalse := false

		_, _, err := validateFieldUpdateRules(dataModelWithUnique, field, table, models.UpdateFieldInput{
			IsUnique: &newFalse,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "cannot remove unicity constraint on the object_id field", "expected error should be returned")
	})

	t.Run("cannot make unique if the field is enum", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "status",
			DataType: models.String,
			IsEnum:   true,
		}
		newTrue := true

		_, _, err := validateFieldUpdateRules(dataModel, field, table, models.UpdateFieldInput{
			IsUnique: &newTrue,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "cannot make a field unique if it is an enum", "expected error should be returned")
	})

	t.Run("field must remain unique if it is the target of a link", func(t *testing.T) {
		asserts := assert.New(t)
		field := models.FieldMetadata{
			Name:     "object_id",
			DataType: models.String,
		}
		table := models.TableMetadata{
			Name:           "accounts",
			OrganizationID: organizationId,
		}
		newFalse := false

		_, _, err := validateFieldUpdateRules(dataModelWithUnique, field, table, models.UpdateFieldInput{
			IsUnique: &newFalse,
		})
		asserts.Error(err, "error expected")
		asserts.ErrorContains(err, "cannot remove unicity constraint on a field that is linked to another table", "expected error should be returned")
	})
}
