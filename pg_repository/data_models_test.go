package pg_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"

	"marble/marble-backend/app"
)

type dataModelTestCase struct {
	name           string
	input          app.DataModel
	expectedOutput interface{}
}

func TestDataModelRepoEndToEnd(t *testing.T) {
	transactions := app.Table{
		Name: "transactions",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"value":       {DataType: app.Float},
			"isValidated": {DataType: app.Bool},
			"account_id":  {DataType: app.String},
		},
		LinksToSingle: map[app.LinkName]app.LinkToSingle{
			"accountss": {
				LinkedTableName: "accountss",
				ParentFieldName: "object_id",
				ChildFieldName:  "accounts_id",
			},
		},
	}
	accountss := app.Table{
		Name: "accountss_test",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":   {DataType: app.Timestamp},
			"status":       {DataType: app.String},
			"is_validated": {DataType: app.Bool},
		},
		LinksToSingle: map[app.LinkName]app.LinkToSingle{},
	}

	dataModel := app.DataModel{
		Tables: map[app.TableName]app.Table{
			"transactions": transactions,
			"accountss":    accountss,
		},
		Version: "1.0.0",
	}
	ctx := context.Background()

	cases := []dataModelTestCase{
		{
			name:           "Read boolean field from DB without join",
			input:          dataModel,
			expectedOutput: dataModel,
		},
	}

	orgID := globalTestParams.testIds["OrganizationId"]
	asserts := assert.New(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			val, err := globalTestParams.repository.CreateDataModel(ctx, orgID, dataModel)
			if err != nil {
				t.Errorf("Could not create data model: %s", err)
			}

			asserts.Equal(c.expectedOutput, val, "[Create] Output data model should match the input one")

			val, err = globalTestParams.repository.GetDataModel(ctx, orgID)
			if err != nil {
				t.Errorf("Could not read data model from DB: %s", err)
			}
			asserts.Equal(c.expectedOutput, val, "[Get] Output data model should match the input one")

			unknownOrgID, _ := uuid.NewV4()
			val, err = globalTestParams.repository.GetDataModel(ctx, unknownOrgID.String())
			if !errors.Is(err, app.ErrNotFoundInRepository) {
				t.Errorf("Should return an error if the org id is unknown: %s", err)
			}
		})

	}

}
