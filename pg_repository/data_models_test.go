package pg_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"

	"marble/marble-backend/models"
)

type dataModelTestCase struct {
	name           string
	input          models.DataModel
	expectedOutput interface{}
}

func TestDataModelRepoEndToEnd(t *testing.T) {
	t.SkipNow()
	transactions := models.Table{
		Name: "transactions",
		Fields: map[models.FieldName]models.Field{
			"object_id": {
				DataType: models.String,
			},
			"updated_at":  {DataType: models.Timestamp},
			"value":       {DataType: models.Float},
			"isValidated": {DataType: models.Bool},
			"account_id":  {DataType: models.String},
		},
		LinksToSingle: map[models.LinkName]models.LinkToSingle{
			"accounts": {
				LinkedTableName: "accounts",
				ParentFieldName: "object_id",
				ChildFieldName:  "account_id",
			},
		},
	}
	accounts := models.Table{
		Name: "accounts_test",
		Fields: map[models.FieldName]models.Field{
			"object_id": {
				DataType: models.String,
			},
			"updated_at":   {DataType: models.Timestamp},
			"status":       {DataType: models.String},
			"is_validated": {DataType: models.Bool},
		},
		LinksToSingle: map[models.LinkName]models.LinkToSingle{},
	}

	dataModel := models.DataModel{
		Tables: map[models.TableName]models.Table{
			"transactions": transactions,
			"accounts":     accounts,
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
			if !errors.Is(err, models.NotFoundInRepositoryError) {
				t.Errorf("Should return an error if the org id is unknown: %s", err)
			}
		})

	}

}
