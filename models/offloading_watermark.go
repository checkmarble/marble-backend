package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type WatermarkType string

const (
	WatermarkTypeDecisionRules WatermarkType = "decision_rules"
	WatermarkTypeMetrics       WatermarkType = "metrics"
)

func (t WatermarkType) String() string {
	return string(t)
}

func WatermarkTypeFromString(s string) (WatermarkType, error) {
	switch s {
	case "decision_rules":
		return WatermarkTypeDecisionRules, nil
	case "metrics":
		return WatermarkTypeMetrics, nil
	default:
		return "", errors.New("invalid watermark type")
	}
}

type Watermark struct {
	Id            uuid.UUID
	OrgId         *string
	Type          WatermarkType
	WatermarkTime time.Time
	WatermarkId   *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Params        *json.RawMessage
}
