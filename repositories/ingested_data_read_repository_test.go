package repositories

// func TestReadFromDb(t *testing.T) {
// 	transactions := models.Table{
// 		Name: "transactions",
// 		Fields: map[models.FieldName]models.Field{
// 			"object_id": {
// 				DataType: models.String,
// 			},
// 			"updated_at": {DataType: models.Timestamp},
// 			"value":      {DataType: models.Float},
// 			"title":      {DataType: models.String},
// 			"account_id": {DataType: models.String},
// 		},
// 		LinksToSingle: map[models.LinkName]models.LinkToSingle{
// 			"accounts": {
// 				LinkedTableName: "accounts",
// 				ParentFieldName: "object_id",
// 				ChildFieldName:  "account_id",
// 			},
// 		},
// 	}
// 	accounts := models.Table{
// 		Name: "accounts",
// 		Fields: map[models.FieldName]models.Field{
// 			"object_id": {
// 				DataType: models.String,
// 			},
// 			"updated_at": {DataType: models.Timestamp},
// 			"name":       {DataType: models.String},
// 			"balance":    {DataType: models.Float},
// 			"company_id": {DataType: models.String},
// 		},
// 		LinksToSingle: map[models.LinkName]models.LinkToSingle{
// 			"companies": {
// 				LinkedTableName: "companies",
// 				ParentFieldName: "object_id",
// 				ChildFieldName:  "company_id",
// 			},
// 		},
// 	}
// 	companies := models.Table{
// 		Name: "companies",
// 		Fields: map[models.FieldName]models.Field{
// 			"object_id": {
// 				DataType: models.String,
// 			},
// 			"updated_at": {DataType: models.Timestamp},
// 			"name":       {DataType: models.String},
// 		},
// 		LinksToSingle: map[models.LinkName]models.LinkToSingle{},
// 	}
// 	dataModel := models.DataModel{
// 		Tables: map[models.TableName]models.Table{
// 			"transactions": transactions,
// 			"accounts":     accounts,
// 			"companies":    companies,
// 		},
// 	}
// 	transactionId := globalTestParams.testIds["TransactionId"]
// 	payload, err := models.ParseToDataModelObject(transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, transactionId)))
// 	if err != nil {
// 		t.Fatalf("Could not parse payload: %s", err)
// 	}
// 	payloadNotInDB, err := models.ParseToDataModelObject(transactions, []byte(fmt.Sprintf(`{"object_id": "%s", "updated_at": "2021-01-01T00:00:00Z"}`, "unknown transactionId")))
// 	if err != nil {
// 		t.Fatalf("Could not parse payload: %s", err)
// 	}

// 	type testCase struct {
// 		name           string
// 		readParams     models.DbFieldReadParams
// 		expectedOutput interface{}
// 		expectedError  error
// 	}

// 	cases := []testCase{
// 		{
// 			name:           "Read string field from DB with one join",
// 			readParams:     models.DbFieldReadParams{TriggerTableName: models.TableName("transactions"), Path: []models.LinkName{"accounts"}, FieldName: "name", DataModel: dataModel, Payload: payload},
// 			expectedOutput: pgtype.Text{String: "SHINE", Valid: true},
// 			expectedError:  nil,
// 		},
// 		{
// 			name:           "Read string field from DB with two joins",
// 			readParams:     models.DbFieldReadParams{TriggerTableName: models.TableName("transactions"), Path: []models.LinkName{"accounts", "companies"}, FieldName: "name", DataModel: dataModel, Payload: payload},
// 			expectedOutput: pgtype.Text{String: "Test company 1", Valid: true},
// 			expectedError:  nil,
// 		},
// 		{
// 			name:           "Read string field from DB, no line found",
// 			readParams:     models.DbFieldReadParams{TriggerTableName: models.TableName("transactions"), Path: []models.LinkName{"accounts"}, FieldName: "name", DataModel: dataModel, Payload: payloadNotInDB},
// 			expectedOutput: pgtype.Text{String: "", Valid: false},
// 			expectedError:  operators.OperatorNoRowsReadInDbError,
// 		},
// 	}

// 	asserts := assert.New(t)
// 	for _, c := range cases {
// 		t.Run(c.name, func(t *testing.T) {
// 			val, err := globalTestParams.repository.GetDbField(context.Background(), c.readParams)

// 			if err != nil {
// 				asserts.True(errors.Is(err, c.expectedError), "Expected error %s, got %s", c.expectedError, err)
// 			}
// 			asserts.Equal(c.expectedOutput, val)

// 		})
// 	}
// }
