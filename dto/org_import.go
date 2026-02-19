package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type ArchetypeDto struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type ArchetypesDto struct {
	Archetypes []ArchetypeDto `json:"archetypes"`
}

func AdaptArchetypeDto(a models.ArchetypeInfo) ArchetypeDto {
	return ArchetypeDto{
		Name:        a.Name,
		Label:       a.Label,
		Description: a.Description,
	}
}

func AdaptArchetypesDto(archetypes []models.ArchetypeInfo) ArchetypesDto {
	return ArchetypesDto{
		Archetypes: pure_utils.Map(archetypes, AdaptArchetypeDto),
	}
}

type OrgImportMetadata struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type OrgImport struct {
	Org         ImportOrg          `json:"org" binding:"required"`
	Admins      []CreateUser       `json:"admins"`
	DataModel   ImportDataModel    `json:"data_model"`
	Scenarios   []ImportScenario   `json:"scenarios"`
	Tags        []ImportTag        `json:"tags"`
	CustomLists []ImportCustomList `json:"custom_lists"`
	Inboxes     []InboxDto         `json:"inboxes"`
	Workflows   []ImportWorkflow   `json:"workflows"`

	Seeds ImportSeeds `json:"seeds"`
}

type ImportOrg struct {
	Name string `json:"name" binding:"required"`
	UpdateOrganizationBodyDto
}

type ImportDataModel struct {
	Tables            []Table                                `json:"tables"`
	Links             []LinkToSingle                         `json:"links"`
	Pivots            []Pivot                                `json:"pivots"`
	NavigationOptions map[string]CreateNavigationOptionInput `json:"navigation_options"`
}

type ImportTag struct {
	CreateTagBody

	Id string `json:"id"`
}

type ImportCustomList struct {
	CustomList

	Values []string `json:"values"`
}

type ImportScenario struct {
	Scenario  ScenarioDto              `json:"scenario"`
	Iteration ScenarioIterationBodyDto `json:"iteration"`
}

type ImportWorkflow struct {
	WorkflowRuleDto

	ScenarioId uuid.UUID              `json:"scenario_id"`
	Conditions []WorkflowConditionDto `json:"conditions"`
	Actions    []WorkflowActionDto    `json:"actions"`
}

type ImportSeeds struct {
	Ingestion map[string]ImportSeedsIngestion `json:"ingestion"`
	Decisions map[string]int                  `json:"decisions"`
}

type ImportSeedsIngestion struct {
	Table  string                      `json:"table"`
	Count  int                         `json:"count"`
	Fields map[string]ImportSeedsField `json:"fields"`
}

type ImportSeedsField struct {
	Ref        string    `json:"ref"`
	Constant   any       `json:"constant"`
	Enum       []any     `json:"enum"`
	IntRange   []int     `json:"int_range"`
	FloatRange []float64 `json:"float_range"`
	Generator  string    `json:"generator"`
	Cast       string    `json:"cast"`
}
