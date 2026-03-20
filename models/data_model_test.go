package models

import (
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
