package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func graphTestField(names ...string) map[string]Field {
	fields := map[string]Field{}
	for _, name := range append([]string{"object_id"}, names...) {
		fields[name] = Field{Name: name, DataType: String}
	}
	return fields
}

// A party owning accounts, accounts owning transactions, and a table nothing links to.
func graphTestModel() DataModel {
	return DataModel{Tables: map[string]Table{
		"users": {
			Name: "users", Fields: graphTestField("email"),
			LinksToSingle: map[string]LinkToSingle{},
		},
		"accounts": {
			Name: "accounts", Fields: graphTestField("user_id", "iban"),
			LinksToSingle: map[string]LinkToSingle{
				"accounts_user": {
					Id: "accounts_user", Name: "accounts_user",
					ChildTableName: "accounts", ChildFieldName: "user_id",
					ParentTableName: "users", ParentFieldName: "object_id",
				},
			},
		},
		"notes": {
			Name: "notes", Fields: graphTestField("body"),
			LinksToSingle: map[string]LinkToSingle{},
		},
	}}
}

func TestGraphTraversableFields(t *testing.T) {
	relations := []GraphRelation{
		{Label: "same_iban", LeftType: "accounts", LeftField: "iban", RightType: "accounts", RightField: "iban"},
	}

	fields := GraphTraversableFields(graphTestModel(), relations)

	// Both ends of the link, plus the relation endpoint.
	assert.Equal(t, []string{"object_id"}, fields["users"])
	assert.Equal(t, []string{"iban", "user_id"}, fields["accounts"])
	assert.Empty(t, fields["notes"], "nothing links to notes and no relation names it")
}

func TestGraphTraversableFields_SkipsRelationsThatDoNotMatchTheDataModel(t *testing.T) {
	// A relation is validated on creation, so one that no longer resolves means the data model
	// drifted since. The walk cannot follow it, so the builder must not index for it either.
	relations := []GraphRelation{
		{Label: "same_iban", LeftType: "accounts", LeftField: "iban", RightType: "accounts", RightField: "iban"},
		{Label: "gone_table", LeftType: "gadgets", LeftField: "ip", RightType: "gadgets", RightField: "ip"},
		{Label: "gone_field", LeftType: "accounts", LeftField: "nope", RightType: "accounts", RightField: "nope"},
	}

	fields := GraphTraversableFields(graphTestModel(), relations)

	assert.Equal(t, []string{"iban", "user_id"}, fields["accounts"])
	assert.Empty(t, fields["gadgets"])
}

// indexedNames flattens the resolved fields back to names, for assertions about the set.
func indexedNames(indexed map[string][]Field, recordType string) []string {
	names := make([]string, 0, len(indexed[recordType]))
	for _, field := range indexed[recordType] {
		names = append(names, field.Name)
	}
	return names
}

func TestGraphIndexedFields_IsASupersetOfWhatAWalkReads(t *testing.T) {
	// This is the invariant the two functions exist to hold: a field a walk reads but the
	// adjacency table does not carry does not fail, it silently finds nothing.
	relations := []GraphRelation{
		{Label: "same_iban", LeftType: "accounts", LeftField: "iban", RightType: "accounts", RightField: "iban"},
		{Label: "same_email", LeftType: "users", LeftField: "email", RightType: "users", RightField: "email"},
	}
	dataModel := graphTestModel()

	traversable := GraphTraversableFields(dataModel, relations)
	indexed := GraphIndexedFields(dataModel, relations)

	for recordType, fields := range traversable {
		for _, field := range fields {
			assert.Contains(t, indexedNames(indexed, recordType), field,
				"%s.%s is readable by a walk but would not be indexed", recordType, field)
		}
	}
}

func TestGraphIndexedFields_CarriesObjectIdOnEveryTable(t *testing.T) {
	indexed := GraphIndexedFields(graphTestModel(), nil)

	for _, recordType := range []string{"users", "accounts", "notes"} {
		assert.Contains(t, indexedNames(indexed, recordType), "object_id",
			"%s must be identifiable even when nothing links to it", recordType)
	}
	// Sorted, so two builds over an unchanged data model emit identical statements.
	assert.Equal(t, []string{"object_id", "user_id"}, indexedNames(indexed, "accounts"))
}

func TestGraphIndexedFields_ResolvesTheDataTypeOfEachField(t *testing.T) {
	// How a value is rendered as text depends on its type, so the builder needs the resolved
	// field rather than just its name.
	dataModel := graphTestModel()
	table := dataModel.Tables["accounts"]
	table.Fields["last_seen_at"] = Field{Name: "last_seen_at", DataType: Timestamp}
	dataModel.Tables["accounts"] = table

	relations := []GraphRelation{
		{Label: "same_seen", LeftType: "accounts", LeftField: "last_seen_at", RightType: "accounts", RightField: "last_seen_at"},
	}

	byName := map[string]DataType{}
	for _, field := range GraphIndexedFields(dataModel, relations)["accounts"] {
		byName[field.Name] = field.DataType
	}

	assert.Equal(t, Timestamp, byName["last_seen_at"])
	assert.Equal(t, String, byName["object_id"])
}

func TestGraphRelation_AppliesTo(t *testing.T) {
	dataModel := graphTestModel()

	tests := []struct {
		name     string
		relation GraphRelation
		expected bool
	}{
		{
			name:     "both endpoints resolve",
			relation: GraphRelation{LeftType: "accounts", LeftField: "iban", RightType: "users", RightField: "email"},
			expected: true,
		},
		{
			name:     "unknown table",
			relation: GraphRelation{LeftType: "gadgets", LeftField: "ip", RightType: "users", RightField: "email"},
		},
		{
			name:     "unknown field on the right",
			relation: GraphRelation{LeftType: "accounts", LeftField: "iban", RightType: "users", RightField: "nope"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.relation.AppliesTo(dataModel))
		})
	}
}

func TestGraphRelation_Endpoints(t *testing.T) {
	self := GraphRelation{LeftType: "accounts", LeftField: "iban", RightType: "accounts", RightField: "iban"}
	require.Len(t, self.Endpoints(), 1, "a self-relation has one endpoint, not two identical ones")

	cross := GraphRelation{LeftType: "accounts", LeftField: "iban", RightType: "transactions", RightField: "sender_iban"}
	assert.Len(t, cross.Endpoints(), 2)
}
