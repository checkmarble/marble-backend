package core

import "github.com/checkmarble/marble-backend/models"

// FieldSemanticType returns the semantic type the client gave to a column of
// its data model (an IBAN, a country, a monetary amount...), or the empty
// semantic type when the table or the field is unknown. It knows nothing of any
// format: the engine and the format hooks use it to read what a column means.
func FieldSemanticType(dm models.DataModel, table, field string) models.FieldSemanticType {
	t, ok := dm.Tables[table]
	if !ok {
		return models.FieldSemanticTypeUnset
	}
	f, ok := t.Fields[field]
	if !ok {
		return models.FieldSemanticTypeUnset
	}
	return f.SemanticType
}
