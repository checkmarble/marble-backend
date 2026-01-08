package repositories

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

func TestAnalyticsCopyScreenings(t *testing.T) {
	req := AnalyticsCopyRequest{
		OrgId:         utils.TextToUUID("123"),
		TriggerObject: "test",
		EndTime:       time.Now(),
		Limit:         10000,
		Watermark: &models.Watermark{
			WatermarkTime: time.Now(),
			WatermarkId:   utils.Ptr(uuid.New().String()),
		},
	}

	inner, err := generateScreeningsExportQuery(req, nil)
	if err != nil {
		t.Fatalf("failed to generate query: %v", err)
	}

	innerSql, args, err := inner.ToSql()
	if err != nil {
		t.Fatalf("failed to build query: %v", err)
	}

	t.Error("just need the log really")
	t.Logf("query: %s", innerSql)
	t.Logf("args: %v", args)
}
