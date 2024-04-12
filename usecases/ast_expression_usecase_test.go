package usecases

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestAstExpressionUsecase_getLinkedDatabaseIdentifiers(t *testing.T) {
	scenario := models.Scenario{
		TriggerObjectType: "transactions",
	}

	model := models.DataModel{
		Tables: map[models.TableName]models.Table{
			"accounts": {
				Name: "accounts",
				Fields: map[models.FieldName]models.Field{
					"id":                  {},
					"last_transaction_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"last_transactions": {
						LinkedTableName: "transactions",
						ParentFieldName: "id",
						ChildFieldName:  "last_transaction",
					},
				},
			},
			"transactions": {
				Name: "transactions",
				Fields: map[models.FieldName]models.Field{
					"id":         {},
					"account_id": {},
				},
				LinksToSingle: map[string]models.LinkToSingle{
					"account": {
						LinkedTableName: "accounts",
						ParentFieldName: "id",
						ChildFieldName:  "account_id",
					},
				},
			},
		},
	}

	u := AstExpressionUsecase{}
	_, err := u.getLinkedDatabaseIdentifiers(scenario, model)
	assert.NoError(t, err)
}
