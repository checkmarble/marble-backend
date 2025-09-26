package analytics

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	TriggerObjectFieldPrefix = "tr."
	DatabaseFieldPrefix      = "ex."
)

type Settings struct {
	Id                uuid.UUID
	TriggerObjectType string
	TriggerFields     []string
	DbFields          []SettingsDbField
}

type SettingsDbField struct {
	Path []string
	Name string
}

func (f SettingsDbField) Ident() string {
	return fmt.Sprintf("%s.%s", strings.Join(f.Path, "."), f.Name)
}
