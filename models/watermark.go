package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WatermarkType string

const (
	WatermarkTypeDecisionRules WatermarkType = "decision_rules"
	WatermarkTypeMetrics       WatermarkType = "metrics"

	WatermarkTypeAnalyticsDecisions     WatermarkType = "analytics_decisions"
	WatermarkTypeAnalyticsDecisionRules WatermarkType = "analytics_decision_rules"
	WatermarkTypeAnalyticsScreenings    WatermarkType = "analytics_screenings"

	WatermarkTypeMergedAnalyticsDecisions     WatermarkType = "analytics_merged_decisions"
	WatermarkTypeMergedAnalyticsDecisionRules WatermarkType = "analytics_merged_decision_rules"
	WatermarkTypeMergedAnalyticsScreenings    WatermarkType = "analytics_merged_screenings"
)

func (t WatermarkType) String() string {
	return string(t)
}

func WatermarkTypeFromString(s string) (WatermarkType, error) {
	// Strip the specialization, if any.
	s, _, _ = strings.Cut(s, ":")

	switch s {
	case "decision_rules":
		return WatermarkTypeDecisionRules, nil
	case "metrics":
		return WatermarkTypeMetrics, nil
	case "analytics_decisions":
		return WatermarkTypeAnalyticsDecisions, nil
	case "analytics_decision_rules":
		return WatermarkTypeAnalyticsDecisionRules, nil
	case "analytics_screenings":
		return WatermarkTypeAnalyticsScreenings, nil
	case "analytics_merged_decisions":
		return WatermarkTypeMergedAnalyticsDecisions, nil
	case "analytics_merged_decision_rules":
		return WatermarkType(WatermarkTypeMergedAnalyticsDecisionRules.String()), nil
	case "analytics_merged_screenings":
		return WatermarkTypeMergedAnalyticsScreenings, nil
	default:
		return "", errors.New("invalid watermark type")
	}
}

// A specialized watermark is a special subtype of a watermark that can be used
// to split a watermark into several independent ones.
func SpecializedWatermark(wm WatermarkType, spec string) WatermarkType {
	return WatermarkType(string(wm) + ":" + spec)
}

type Watermark struct {
	Id            uuid.UUID
	OrgId         *string
	Type          WatermarkType
	WatermarkTime time.Time
	WatermarkId   *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Params        json.RawMessage
}
