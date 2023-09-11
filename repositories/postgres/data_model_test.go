package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pashagolub/pgxmock/v2"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

var exampleDataModel = models.DataModel{
	Version: "version",
	Status:  models.Live,
	Tables: map[models.TableName]models.Table{
		"table": {
			Name: "table",
			Fields: map[models.FieldName]models.Field{
				"integer": {
					DataType: models.Int,
					Nullable: false,
				},
			},
		},
	},
}

func TestDatabase_GetDataModel(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		rawTables, err := json.Marshal(&exampleDataModel.Tables)
		assert.NoError(t, err)

		mock.ExpectQuery("SELECT id, org_id, version, status, tables, deleted_at FROM data_models").
			WithArgs("organizationID").
			WillReturnRows(pgxmock.NewRows([]string{"id", "org_id", "version", "status", "tables", "deleted_at"}).
				AddRow("id", "org_id", "version", "live", rawTables, nil),
			)

		database := Database{
			pool: mock,
		}

		model, err := database.GetDataModel(context.Background(), "organizationID")
		assert.NoError(t, err)
		assert.Equal(t, exampleDataModel, model)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		mock.ExpectQuery("SELECT id, org_id, version, status, tables, deleted_at FROM data_models").
			WithArgs("organizationID").
			WillReturnError(assert.AnError)

		database := Database{
			pool: mock,
		}

		_, err = database.GetDataModel(context.Background(), "organizationID")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDatabase_CreateDataModel(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		rawTables, err := json.Marshal(&exampleDataModel.Tables)
		assert.NoError(t, err)

		mock.ExpectExec("INSERT INTO data_models").
			WithArgs("organizationID", exampleDataModel.Version, exampleDataModel.Status.String(), rawTables).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		database := Database{
			pool: mock,
		}

		err = database.CreateDataModel(context.Background(), "organizationID", exampleDataModel)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("exec error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		rawTables, err := json.Marshal(&exampleDataModel.Tables)
		assert.NoError(t, err)

		mock.ExpectExec("INSERT INTO data_models").
			WithArgs("organizationID", exampleDataModel.Version, exampleDataModel.Status.String(), rawTables).
			WillReturnError(assert.AnError)

		database := Database{
			pool: mock,
		}

		err = database.CreateDataModel(context.Background(), "organizationID", exampleDataModel)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDatabase_DeleteDataModel(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		mock.ExpectExec("DELETE FROM data_models").
			WithArgs("organizationID").
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		database := Database{
			pool: mock,
		}

		err = database.DeleteDataModel(context.Background(), "organizationID")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("exec error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatal(err)
		}
		defer mock.Close()

		mock.ExpectExec("DELETE FROM data_models").
			WithArgs("organizationID").
			WillReturnError(assert.AnError)

		database := Database{
			pool: mock,
		}

		err = database.DeleteDataModel(context.Background(), "organizationID")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
