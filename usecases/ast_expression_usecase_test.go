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
		"transactions-[account last_transactions]-id",
		"transactions-[account last_transactions]-account_id",
	}
	sort.Strings(expectedStr)
	indentifiersStr := pure_utils.Map(identifiers, dbAccessNodeToString)
	sort.Strings(indentifiersStr)
	assert.Equal(t, indentifiersStr, expectedStr)
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
	assert.Equal(t, indentifiersStr, expectedStr)
}
