package pg_repository

import (
	"context"
	"fmt"
	"testing"

	"marble/marble-backend/app"

	sq "github.com/Masterminds/squirrel"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestHandleFirstIngestObject(t *testing.T) {
	transactions := app.Table{
		Name: "transactions",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"amount":      {DataType: app.Float},
			"title":       {DataType: app.String},
			"accounts_id": {DataType: app.String},
		},
	}
	ctx := context.Background()
	logger := globalTestParams.logger

	object_id, err := uuid.NewV4()
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, object_id.String())))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}

	assert := assert.New(t)
	err = globalTestParams.repository.IngestObject(ctx, payload, transactions, logger)
	if err != nil {
		t.Errorf("Error while inserting object into DB: %s", err)
	}

	sql, args, err := globalTestParams.repository.queryBuilder.
		Select("COUNT(*) AS nb").
		From(string(transactions.Name)).
		Where(sq.Eq{"object_id": payload.ReadFieldFromDynamicStruct("object_id")}).
		ToSql()
	var nb int
	_ = globalTestParams.repository.db.QueryRow(ctx, sql, args...).Scan(&nb)
	assert.Equal(1, nb, "Expected to find 1 row in DB")

	sql, args, err = globalTestParams.repository.queryBuilder.
		Select("valid_from, valid_until").
		From(string(transactions.Name)).
		Where(sq.Eq{"object_id": payload.ReadFieldFromDynamicStruct("object_id")}).
		ToSql()
	var valid_from, valid_until pgtype.Timestamp
	_ = globalTestParams.repository.db.QueryRow(ctx, sql, args...).Scan(&valid_from, &valid_until)
	assert.True(valid_from.Valid, "Expected valid_from to be valid")
	assert.True(valid_until.Valid, "Expected valid_until to be valid")
	assert.Equal(pgtype.Infinity, valid_until.InfinityModifier, "Expected valid_until to be infinity")

}

func TestHandleRenewedIngestObject(t *testing.T) {
	transactions := app.Table{
		Name: "transactions",
		Fields: map[app.FieldName]app.Field{
			"object_id": {
				DataType: app.String,
			},
			"updated_at":  {DataType: app.Timestamp},
			"amount":      {DataType: app.Float},
			"title":       {DataType: app.String},
			"accounts_id": {DataType: app.String},
		},
	}
	ctx := context.Background()
	logger := globalTestParams.logger

	object_id, err := uuid.NewV4()
	payload, err := app.ParseToDataModelObject(ctx, transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, object_id.String())))
	if err != nil {
		t.Fatalf("Could not parse payload: %s", err)
	}

	assert := assert.New(t)
	err = globalTestParams.repository.IngestObject(ctx, payload, transactions, logger)
	if err != nil {
		t.Errorf("Error while inserting object into DB: %s", err)
	}
	_ = globalTestParams.repository.IngestObject(ctx, payload, transactions, logger)

	sql, args, err := globalTestParams.repository.queryBuilder.
		Select("COUNT(*) AS nb").
		From(string(transactions.Name)).
		Where(sq.Eq{"object_id": payload.ReadFieldFromDynamicStruct("object_id")}).
		ToSql()
	var nb int
	_ = globalTestParams.repository.db.QueryRow(ctx, sql, args...).Scan(&nb)
	assert.Equal(2, nb, "Expected to find 2 rows in DB")

	sql, args, err = globalTestParams.repository.queryBuilder.
		Select("valid_from, valid_until").
		From(string(transactions.Name)).
		Where(sq.Eq{"object_id": payload.ReadFieldFromDynamicStruct("object_id")}).
		OrderBy("valid_from").
		ToSql()
	var valid_from, valid_until pgtype.Timestamp
	rows, err := globalTestParams.repository.db.Query(ctx, sql, args...)
	rows.Next()
	rows.Scan(&valid_from, &valid_until)
	assert.True(valid_from.Valid, "Expected valid_from to be valid")
	assert.True(valid_until.Valid, "Expected valid_until to be valid")
	assert.Equal(pgtype.Finite, valid_until.InfinityModifier, "Expected valid_until for first row to be finite")

	rows.Next()
	rows.Scan(&valid_from, &valid_until)
	assert.True(valid_from.Valid, "Expected valid_from to be valid")
	assert.True(valid_until.Valid, "Expected valid_until to be valid")
	assert.Equal(pgtype.Infinity, valid_until.InfinityModifier, "Expected valid_until for second row to be Infinite")

}
