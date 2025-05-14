package usecases

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

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
	scenario := models.Scenario{
		TriggerObjectType: "transactions",
	}

	model := models.DataModel{
		Tables: map[string]models.Table{
			"accounts": {
				Name: "accounts",
				Fields: map[string]models.Field{
					"id":                  {},
					"last_transaction_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"last_transactions": {
						ParentTableName: "transactions",
						ParentFieldName: "id",
						ChildFieldName:  "last_transaction",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]models.Field{
					"id":         {},
					"account_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						ParentTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
				},
			},
		},
	}

	identifiers, err := getLinkedDatabaseIdentifiers(scenario, model)
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
	scenario := models.Scenario{
		TriggerObjectType: "transactions",
	}

	model := models.DataModel{
		Tables: map[string]models.Table{
			"companies": {
				Name: "companies",
				Fields: map[string]models.Field{
					"id": {},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]models.Field{
					"id":         {},
					"company_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"company": {
						ParentTableName: "companies",
						ParentFieldName: "id",
						ChildFieldName:  "company_id",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[string]models.Field{
					"id":         {},
					"account_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
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

	identifiers, err := getLinkedDatabaseIdentifiers(scenario, model)
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
	scenario := models.Scenario{
		TriggerObjectType: "transactions",
	}

	model := models.DataModel{
		Tables: map[string]models.Table{
			"projects": {
				Name: "projects",
				Fields: map[string]models.Field{
					"id": {},
				},
			},
			"account_holders": {
				Name: "account_holders",
				Fields: map[string]models.Field{
					"id":         {},
					"project_id": {},
					"aml_score":  {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"project": {
						ParentTableName: "projects",
						ParentFieldName: "id",
						ChildFieldName:  "project_id",
					},
				},
			},
			"accounts": {
				Name: "accounts",
				Fields: map[string]models.Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
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
				Fields: map[string]models.Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
					"account_id":        {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
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
				Fields: map[string]models.Field{
					"id":                {},
					"project_id":        {},
					"account_holder_id": {},
					"account_id":        {},
					"card_id":           {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
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

	identifiers, err := getLinkedDatabaseIdentifiers(scenario, model)
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
