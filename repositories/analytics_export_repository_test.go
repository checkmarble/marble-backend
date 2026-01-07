package repositories

// func TestAnalyticsCopyScreenings(t *testing.T) {
// 	req := AnalyticsCopyRequest{
// 		OrgId:         "123",
// 		TriggerObject: "test",
// 		EndTime:       time.Now(),
// 		Limit:         10000,
// 		Watermark: &models.Watermark{
// 			WatermarkTime: time.Now(),
// 			WatermarkId:   utils.Ptr(uuid.New().String()),
// 		},
// 	}

// 	inner, err := generateScreeningsExportQuery(req, nil, nil)
// 	if err != nil {
// 		t.Fatalf("failed to generate query: %v", err)
// 	}

// 	innerSql, args, err := inner.ToSql()
// 	if err != nil {
// 		t.Fatalf("failed to build query: %v", err)
// 	}

// 	t.Error("just need the log really")
// 	t.Logf("query: %s", innerSql)
// 	t.Logf("args: %v", args)
// }
