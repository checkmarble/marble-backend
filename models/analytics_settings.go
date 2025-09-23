package models

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type AnalyticsSettings struct {
	Id                uuid.UUID
	TriggerObjectType string
	TriggerFields     []string
	DbFields          []AnalyticsSettingsDbField
}

type AnalyticsSettingsDbField struct {
	Path []string
	Name string
}

func (f AnalyticsSettingsDbField) Ident() string {
	return fmt.Sprintf("%s.%s", strings.Join(f.Path, "."), f.Name)
}

// type DecisionAnalyticsField struct {
// 	Type  DataType `json:"type"`
// 	Value any      `json:"value"`
// }
