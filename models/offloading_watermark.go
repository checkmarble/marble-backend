package models

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
)

type WatermarkType string

const (
	WatermarkTypeDecisionRules WatermarkType = "decision_rules"
	WatermarkTypeMetrics       WatermarkType = "metrics"
)

func (t WatermarkType) String() string {
	return string(t)
}

func WaterMarkTypeFromString(s string) (WatermarkType, error) {
	switch s {
	case "decision_rules":
		return WatermarkTypeDecisionRules, nil
	case "metrics":
		return WatermarkTypeMetrics, nil
	default:
		return "", errors.Newf("invalid watermark type: %s", s)
	}
}

type Watermark struct {
	OrgId         *string
	Type          WatermarkType
	WatermarkTime time.Time
	WatermarkId   *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Params        *json.RawMessage
}
