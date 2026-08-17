package models

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/stretchr/testify/assert"
)

func TestDataType(t *testing.T) {
	// DataType is serialized in database
	// So we want to make sure the values stay stable
	assert.Equal(t, int(UnknownDataType), -1)
	assert.Equal(t, int(Bool), 0)
	assert.Equal(t, int(Int), 1)
	assert.Equal(t, int(Float), 2)
	assert.Equal(t, int(String), 3)
	assert.Equal(t, int(Timestamp), 4)
}

func dbAccessNodeToString(node ast.Node) string {
	return fmt.Sprintf("%s-%s-%s",
		node.NamedChildren["tableName"].Constant,
		node.NamedChildren["path"].Constant,
		node.NamedChildren["fieldName"].Constant,
	)
}

func TestAstExpressionUsecase_getLinkedDatabaseIdentifiers_with_loop(t *testing.T) {
	/*
		                +----------------+
		                |  transactions |
		                |               |
		                | id            |
		                | account_id    |
		                +----------------+
		                     ↑   ↓
		                     |   |
		                +----------------+
		                |   accounts     |
		                |               |
		                | id            |
		                | last_trans_id |
		                +----------------+

		Legend:
		↑ : LinksToSingle from transactions to accounts via account.id
		↓ : LinksToSingle from accounts to transactions via last_transactions

		Relationships:
		- transactions → accounts: via account_id → id
		- accounts → transactions: via last_transaction → id
	*/
	scenario := Scenario{
		TriggerObjectType: "transactions",
	}

	model := DataModel{
		Tables: map[string]Table{
			"accounts": {
				Name: "accounts",
				Fields: map[string]Field{
					"id":                  {},
					"last_transaction_id": {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"last_transactions": {
						ParentTableName: "transactions",
						ParentFieldName: "id",
						ChildFieldName:  "last_transaction",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]Field{
					"id":         {},
					"account_id": {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"account": {
						ParentTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
				},
			},
		},
	}

	identifiers, err := GetLinkedDatabaseIdentifiers(scenario, model)
	assert.NoError(t, err)

	expectedStr := []string{
		"transactions-[account]-id",
		"transactions-[account]-last_transaction_id",
		// loops were allowed in a past iteration (as long as any given link was walked only once), but are no longer.
		// "transactions-[account last_transactions]-id",
		// "transactions-[account last_transactions]-account_id",
	}
	sort.Strings(expectedStr)
	indentifiersStr := pure_utils.Map(identifiers, dbAccessNodeToString)
	sort.Strings(indentifiersStr)
	assert.Equal(t, expectedStr, indentifiersStr)
}

func TestAstExpressionUsecase_getLinkedDatabaseIdentifiers_with_two_branches(t *testing.T) {
	/*
		                    +------------+
		                    | companies  |
		                    |            |
		                    | id         |
		                    +------------+
		                     ↑          ↑
		                     |          |
		                     |          |
		                +------------+  |
		                | accounts   |  |
		                |            |  |
		                | id         |  |
		                | company_id |  |
		                +------------+  |
		                     ↑         |
		                     |         |
		                +------------+ |
		                |transactions| |
		                |            | |
		                | id         | |
		                | account_id | |
		                | company_id |-+
		                +------------+

		Legend:
		↑ : Represents LinksToSingle relationship
	*/
	scenario := Scenario{
		TriggerObjectType: "transactions",
	}

	model := DataModel{
		Tables: map[string]Table{
			"companies": {
				Name: "companies",
				Fields: map[string]Field{
					"id": {},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]Field{
					"id":         {},
					"company_id": {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"company": {
						ParentTableName: "companies",
						ParentFieldName: "id",
						ChildFieldName:  "company_id",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]Field{
					"id":         {},
					"account_id": {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"account": {
						ParentTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
					"company": {
						ParentTableName: "companies",
						ParentFieldName: "id",
						ChildFieldName:  "company_id",
					},
				},
			},
		},
	}

	identifiers, err := GetLinkedDatabaseIdentifiers(scenario, model)
	assert.NoError(t, err)

	expectedStr := []string{
		"transactions-[account]-id",
		"transactions-[account]-company_id",
		"transactions-[account company]-id",
		"transactions-[company]-id",
	}
	sort.Strings(expectedStr)
	indentifiersStr := pure_utils.Map(identifiers, dbAccessNodeToString)
	sort.Strings(indentifiersStr)
	assert.Equal(t, expectedStr, indentifiersStr)
}

func TestAstExpressionUsecase_getLinkedDatabaseIdentifiers_with_two_branches_bis(t *testing.T) {
	scenario := Scenario{
		TriggerObjectType: "transactions",
	}

	model := DataModel{
		Tables: map[string]Table{
			"projects": {
				Name: "projects",
				Fields: map[string]Field{
					"id": {},
				},
			},
			"account_holders": {
				Name: "account_holders",
				Fields: map[string]Field{
					"id":         {},
					"project_id": {},
					"aml_score":  {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"project": {
						ParentTableName: "projects",
						ParentFieldName: "id",
						ChildFieldName:  "project_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"projects": {
						ParentTableName: "projects",
						ParentFieldName: "id",
						ChildFieldName:  "project_id",
					},
					"account_holder": {
						ParentTableName: "account_holders",
						ParentFieldName: "id",
						ChildFieldName:  "account_holder_id",
					},
				},
			},
			"cards": {
				Name: "cards",
				Fields: map[string]Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
					"account_id":        {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"projects": {
						ParentTableName: "projects",
						ParentFieldName: "id",
						ChildFieldName:  "project_id",
					},
					"account_holder": {
						ParentTableName: "account_holders",
						ParentFieldName: "id",
						ChildFieldName:  "account_holder_id",
					},
					"account": {
						ParentTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
					"account_id":        {},
					"card_id":           {},
				},
				LinksToSingle: map[string]LinkToSingle{
					"projects": {
						ParentTableName: "projects",
						ParentFieldName: "id",
						ChildFieldName:  "project_id",
					},
					"account_holder": {
						ParentTableName: "account_holders",
						ParentFieldName: "id",
						ChildFieldName:  "account_holder_id",
					},
					"account": {
						ParentTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
					"card": {
						ParentTableName: "cards",
						ParentFieldName: "id",
						ChildFieldName:  "card_id",
					},
				},
			},
		},
	}

	identifiers, err := GetLinkedDatabaseIdentifiers(scenario, model)
	assert.NoError(t, err)

	expectedStr := []string{
		"transactions-[card account account_holder project]-id",
		"transactions-[card account account_holder]-aml_score",
		"transactions-[card account account_holder]-id",
		"transactions-[card account account_holder]-project_id",
		"transactions-[card account projects]-id",
		"transactions-[card account]-account_holder_id",
		"transactions-[card account]-id",
		"transactions-[card account]-project_id",
		"transactions-[card account_holder project]-id",
		"transactions-[card account_holder]-aml_score",
		"transactions-[card account_holder]-id",
		"transactions-[card account_holder]-project_id",
		"transactions-[card projects]-id",
		"transactions-[card]-account_holder_id",
		"transactions-[card]-account_id",
		"transactions-[card]-id",
		"transactions-[card]-project_id",
		"transactions-[account account_holder project]-id",
		"transactions-[account account_holder]-aml_score",
		"transactions-[account account_holder]-id",
		"transactions-[account account_holder]-project_id",
		"transactions-[account projects]-id",
		"transactions-[account]-account_holder_id",
		"transactions-[account]-id",
		"transactions-[account]-project_id",
		"transactions-[account_holder project]-id",
		"transactions-[account_holder]-aml_score",
		"transactions-[account_holder]-id",
		"transactions-[account_holder]-project_id",
		"transactions-[projects]-id",
	}
	sort.Strings(expectedStr)
	indentifiersStr := pure_utils.Map(identifiers, dbAccessNodeToString)
	sort.Strings(indentifiersStr)
	assert.Equal(t, expectedStr, indentifiersStr)
}

func TestFieldWithSemanticSubType(t *testing.T) {
	field := func(name, metadata string) Field {
		return Field{Name: name, Metadata: json.RawMessage(metadata)}
	}

	table := Table{Fields: map[string]Field{
		"object_id":  field("object_id", ""),
		"email":      field("email", `{"hidden": true}`),
		"first_name": field("first_name", `{"semanticSubType": "caption", "hidden": false}`),
		"iban":       field("iban", `{"semanticSubType": "something_else"}`),
		"broken":     field("broken", `not json`),
	}}

	found, ok := table.FieldWithSemanticSubType(FieldSemanticSubTypeCaption)
	assert.True(t, ok)
	assert.Equal(t, "first_name", found.Name)

	_, ok = table.FieldWithSemanticSubType(FieldSemanticSubType("nobody_declares_this"))
	assert.False(t, ok, "a sub-type no field declares resolves to nothing")

	_, ok = table.FieldWithSemanticSubType(FieldSemanticSubTypeUnset)
	assert.False(t, ok, "every field without a sub-type would otherwise match")

	// A table declaring the same sub-type twice must not depend on map iteration order.
	twice := Table{Fields: map[string]Field{
		"b_name": field("b_name", `{"semanticSubType": "caption"}`),
		"a_name": field("a_name", `{"semanticSubType": "caption"}`),
	}}

	for range 20 {
		found, ok := twice.FieldWithSemanticSubType(FieldSemanticSubTypeCaption)
		assert.True(t, ok)
		assert.Equal(t, "a_name", found.Name)
	}
}

func TestFieldSemanticSubType(t *testing.T) {
	// The metadata blob is free-form and written by whoever edits the data model, so a field
	// simply declares no sub-type rather than failing the read.
	for _, metadata := range []string{"", "null", "{}", "not json", `{"semanticSubType": null}`} {
		assert.Equal(t, FieldSemanticSubTypeUnset,
			Field{Metadata: json.RawMessage(metadata)}.SemanticSubType(),
			"metadata %q", metadata)
	}

	assert.Equal(t, FieldSemanticSubTypeCaption,
		Field{Metadata: json.RawMessage(`{"semanticSubType": "caption"}`)}.SemanticSubType())
}
