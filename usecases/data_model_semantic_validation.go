// Define semantic types validation function for data model creation and update.
// Define the pre-check before inserting in persistence layer but also the post-check after retrieving the data model from database before committing to ensure
// that the data model is always consistent with the semantic type rules.
package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

var TableSemanticTypeValidationFunctions = map[models.SemanticType](func(tableName string, datamodel models.DataModel) error){
	models.SemanticTypePerson:      partyTableSemanticTypeValidation,
	models.SemanticTypeCompany:     partyTableSemanticTypeValidation,
	models.SemanticTypeAccount:     belongsToPartyTableSemanticTypeValidation,
	models.SemanticTypeTransaction: belongsToPartyTableSemanticTypeValidation,
	models.SemanticTypeEvent:       belongsToPartyTableSemanticTypeValidation,
	models.SemanticTypePartner:     basicTableSemanticTypeValidation,
	models.SemanticTypeOther:       basicTableSemanticTypeValidation,
}

// Pre-validation function
// Basic validation function, check only if the required fields are present
func basicTableSemanticTypeValidation(tableName string, datamodel models.DataModel) error {
	table, ok := datamodel.Tables[tableName]
	if !ok {
		return errors.Wrap(models.BadParameterError, "table not found in data model")
	}

	objectId, ok := table.Fields["object_id"]
	if !ok {
		return errors.Wrap(models.BadParameterError, "field object_id is required")
	}
	if objectId.DataType != models.String {
		return errors.Wrap(models.BadParameterError,
			"field object_id must be of type String")
	}
	if objectId.Nullable {
		return errors.Wrap(models.BadParameterError,
			"field object_id must be non-nullable")
	}
	if objectId.IsEnum {
		return errors.Wrap(models.BadParameterError,
			"field object_id must not be an enum")
	}

	updatedAt, ok := table.Fields["updated_at"]
	if !ok {
		return errors.Wrap(models.BadParameterError, "field updated_at is required")
	}
	if updatedAt.DataType != models.Timestamp {
		return errors.Wrap(models.BadParameterError,
			"field updated_at must be of type Timestamp")
	}
	if updatedAt.Nullable {
		return errors.Wrap(models.BadParameterError,
			"field updated_at must be non-nullable")
	}
	if updatedAt.IsEnum {
		return errors.Wrap(models.BadParameterError,
			"field updated_at must not be an enum")
	}

	return nil
}

// Check if the table has at least one "Name" field
func partyTableSemanticTypeValidation(tableName string, datamodel models.DataModel) error {
	if err := basicTableSemanticTypeValidation(tableName, datamodel); err != nil {
		return err
	}

	// TODO: in next PR, when dealing with field semantic, add the check on field semantic type

	return nil
}

func belongsToPartyTableSemanticTypeValidation(tableName string, datamodel models.DataModel) error {
	if err := basicTableSemanticTypeValidation(tableName, datamodel); err != nil {
		return err
	}
	table := datamodel.Tables[tableName]

	linkBelongsToName := ""
	for _, link := range table.LinksToSingle {
		if link.LinkType == models.LinkTypeBelongsTo {
			if linkBelongsToName != "" {
				return errors.Wrap(models.BadParameterError,
					"transaction table must have only one BelongsTo link to a Party table")
			}
			linkBelongsToName = link.Name
		}
	}

	if linkBelongsToName == "" {
		return errors.Wrap(models.BadParameterError,
			"transaction table must have a BelongsTo link to a Party table")
	}

	link := table.LinksToSingle[linkBelongsToName]
	linkedTable, ok := datamodel.Tables[link.ParentTableName]
	if !ok {
		return errors.Wrap(models.BadParameterError,
			"linked table not found in data model")
	}
	if !linkedTable.SemanticType.IsParty() {
		return errors.Wrap(models.BadParameterError,
			"transaction table must have a BelongsTo link to a Party table")
	}

	return nil
}
