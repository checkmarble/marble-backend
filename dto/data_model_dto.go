package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

type LinkToSingle struct {
	Id              string `json:"id"`
	ParentTableName string `json:"parent_table_name"`
	ParentTableId   string `json:"parent_table_id"`
	ParentFieldName string `json:"parent_field_name"`
	ParentFieldId   string `json:"parent_field_id"`
	ChildTableName  string `json:"child_table_name"`
	ChildTableId    string `json:"child_table_id"`
	ChildFieldName  string `json:"child_field_name"`
	ChildFieldId    string `json:"child_field_id"`
}

type Field struct {
	ID                string  `json:"id"`
	DataType          string  `json:"data_type"`
	Description       string  `json:"description"`
	IsEnum            bool    `json:"is_enum"`
	Name              string  `json:"name"`
	Nullable          bool    `json:"nullable"`
	TableId           string  `json:"table_id"`
	Values            []any   `json:"values,omitempty"`
	UnicityConstraint string  `json:"unicity_constraint"`
	FTMProperty       *string `json:"ftm_property,omitempty"`
}

type NavigationOption struct {
	SourceTableName   string `json:"source_table_name"`
	SourceTableId     string `json:"source_table_id"`
	SourceFieldName   string `json:"source_field_name"`
	SourceFieldId     string `json:"source_field_id"`
	TargetTableName   string `json:"target_table_name"`
	TargetTableId     string `json:"target_table_id"`
	FilterFieldName   string `json:"filter_field_name"`
	FilterFieldId     string `json:"filter_field_id"`
	OrderingFieldName string `json:"ordering_field_name"`
	OrderingFieldId   string `json:"ordering_field_id"`
	Status            string `json:"status"`
}

type Table struct {
	ID                string                  `json:"id"`
	Name              string                  `json:"name"`
	Description       string                  `json:"description"`
	Fields            map[string]Field        `json:"fields"`
	LinksToSingle     map[string]LinkToSingle `json:"links_to_single,omitempty"`
	NavigationOptions []NavigationOption      `json:"navigation_options,omitempty"`
	FTMEntity         *string                 `json:"ftm_entity,omitempty"`
	Alias             string                  `json:"alias"`
	SemanticType      models.SemanticType     `json:"semantic_type"`
	CaptionField      string                  `json:"caption_field"`
}

type DataModel struct {
	Tables map[string]Table `json:"tables"`
}

func AdaptTableDto(table models.Table) Table {
	var ftmEntity *string
	if table.FTMEntity != nil {
		ftmEntity = utils.Ptr(table.FTMEntity.String())
	}
	return Table{
		Name:              table.Name,
		ID:                table.ID,
		Fields:            pure_utils.MapValues(table.Fields, adaptDataModelField),
		LinksToSingle:     pure_utils.MapValues(table.LinksToSingle, adaptDataModelLink),
		NavigationOptions: pure_utils.Map(table.NavigationOptions, adaptDataModelNavigationOption),
		Description:       table.Description,
		FTMEntity:         ftmEntity,
		Alias:             table.Alias,
		SemanticType:      table.SemanticType,
		CaptionField:      table.CaptionField,
	}
}

func adaptDataModelField(field models.Field) Field {
	var ftmProperty *string
	if field.FTMProperty != nil {
		ftmProperty = utils.Ptr(field.FTMProperty.String())
	}
	return Field{
		ID:                field.ID,
		DataType:          field.DataType.String(),
		Description:       field.Description,
		IsEnum:            field.IsEnum,
		Name:              field.Name,
		Nullable:          field.Nullable,
		TableId:           field.TableId,
		Values:            field.Values,
		UnicityConstraint: field.UnicityConstraint.String(),
		FTMProperty:       ftmProperty,
	}
}

func adaptDataModelLink(linkToSingle models.LinkToSingle) LinkToSingle {
	return LinkToSingle{
		Id:              linkToSingle.Id,
		ParentTableName: linkToSingle.ParentTableName,
		ParentTableId:   linkToSingle.ParentTableId,
		ParentFieldName: linkToSingle.ParentFieldName,
		ParentFieldId:   linkToSingle.ParentFieldId,
		ChildTableName:  linkToSingle.ChildTableName,
		ChildTableId:    linkToSingle.ChildTableId,
		ChildFieldName:  linkToSingle.ChildFieldName,
		ChildFieldId:    linkToSingle.ChildFieldId,
	}
}

func adaptDataModelNavigationOption(navigationOption models.NavigationOption) NavigationOption {
	return NavigationOption{
		SourceTableName:   navigationOption.SourceTableName,
		SourceTableId:     navigationOption.SourceTableId,
		SourceFieldName:   navigationOption.SourceFieldName,
		SourceFieldId:     navigationOption.SourceFieldId,
		TargetTableName:   navigationOption.TargetTableName,
		TargetTableId:     navigationOption.TargetTableId,
		FilterFieldName:   navigationOption.FilterFieldName,
		FilterFieldId:     navigationOption.FilterFieldId,
		OrderingFieldName: navigationOption.OrderingFieldName,
		OrderingFieldId:   navigationOption.OrderingFieldId,
		Status:            navigationOption.Status.String(),
	}
}

func AdaptDataModelDto(dataModel models.DataModel) DataModel {
	return DataModel{
		Tables: pure_utils.MapValues(dataModel.Tables, AdaptTableDto),
	}
}

type CreateTableInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	FTMEntity   *string `json:"ftm_entity"`
}

type UpdateTableInput struct {
	Description  *string                 `json:"description"`
	FTMEntity    pure_utils.Null[string] `json:"ftm_entity"`
	Alias        pure_utils.Null[string] `json:"alias"`
	SemanticType pure_utils.Null[string] `json:"semantic_type"`
	CaptionField pure_utils.Null[string] `json:"caption_field"`
}

type CreateLinkInput struct {
	Name          string `json:"name"`
	ParentTableId string `json:"parent_table_id"`
	ParentFieldId string `json:"parent_field_id"`
	ChildTableId  string `json:"child_table_id"`
	ChildFieldId  string `json:"child_field_id"`
}

type UpdateFieldInput struct {
	Description *string                 `json:"description"`
	IsEnum      *bool                   `json:"is_enum"`
	IsUnique    *bool                   `json:"is_unique"`
	IsNullable  *bool                   `json:"is_nullable"`
	FTMProperty pure_utils.Null[string] `json:"ftm_property"`
}

type CreateFieldInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Nullable    bool    `json:"nullable"`
	IsEnum      bool    `json:"is_enum"`
	IsUnique    bool    `json:"is_unique"`
	FTMProperty *string `json:"ftm_property"`
}

type DataModelObject struct {
	Data     map[string]any `json:"data"`
	Metadata map[string]any `json:"metadata"`
}

type CreateNavigationOptionInput struct {
	SourceFieldId   string `json:"source_field_id"`
	TargetTableId   string `json:"target_table_id"`
	FilterFieldId   string `json:"filter_field_id"`
	OrderingFieldId string `json:"ordering_field_id"`
}

type UpdateDataModelOptionsInput struct {
	DisplayedFields []string `json:"displayed_fields"`
	FieldOrder      []string `json:"field_order"`
}

type DataModelOptions struct {
	DisplayedFields []string `json:"displayed_fields,omitzero"`
	FieldOrder      []string `json:"field_order,omitzero"`
}

func AdaptDataModelOptions(m models.DataModelOptions) DataModelOptions {
	return DataModelOptions{
		DisplayedFields: m.DisplayedFields,
		FieldOrder:      m.FieldOrder,
	}
}

type DataModelDeleteFieldReport struct {
	Performed          bool                          `json:"performed"`
	Conflicts          DataModelDeleteFieldConflicts `json:"conflicts"`
	ArchivedIterations []DataModelDeleteFieldRef     `json:"archived_iterations"`
}

type DataModelDeleteFieldConflicts struct {
	ContinuousScreening bool                                              `json:"continuous_screening"`
	Links               []string                                          `json:"links"`
	Pivots              []string                                          `json:"pivots"`
	AnalyticsSettings   int                                               `json:"analytics_settings"`
	Scenarios           []DataModelDeleteFieldRef                         `json:"scenarios"`
	EmptyScenarios      []DataModelDeleteFieldRef                         `json:"empty_scenarios"`
	ScenarioIterations  map[string]*DataModelDeleteFieldConflictIteration `json:"scenario_iterations"`
	Workflows           []DataModelDeleteFieldRef                         `json:"workflows"`
	TestRuns            bool                                              `json:"test_runs"`
}

type DataModelDeleteFieldRef struct {
	Id    string `json:"id"`
	Label string `json:"label"`
}

type DataModelDeleteFieldConflictIteration struct {
	Name             string                    `json:"name"`
	ScenarioId       string                    `json:"scenario_id"`
	Draft            bool                      `json:"draft"`
	TriggerCondition bool                      `json:"trigger_condition"`
	Rules            []DataModelDeleteFieldRef `json:"rules"`
	Screenings       []DataModelDeleteFieldRef `json:"screenings"`
}

func AdaptDataModelDeleteFieldReport(m models.DataModelDeleteFieldReport) DataModelDeleteFieldReport {
	r := DataModelDeleteFieldReport{
		Performed: m.Performed,
		Conflicts: DataModelDeleteFieldConflicts{
			ContinuousScreening: m.Conflicts.ContinuousScreening,
			Links:               m.Conflicts.Links.Slice(),
			Pivots:              m.Conflicts.Pivots.Slice(),
			AnalyticsSettings:   m.Conflicts.AnalyticsSettings,
			Workflows: pure_utils.Map(m.Conflicts.Workflows.Slice(), func(id string) DataModelDeleteFieldRef {
				return AdaptDataModelDeleteFieldReportRef(m, id)
			}),
			Scenarios: pure_utils.Map(m.Conflicts.Scenario.Slice(), func(id string) DataModelDeleteFieldRef {
				return AdaptDataModelDeleteFieldReportRef(m, id)
			}),
			EmptyScenarios: pure_utils.Map(m.Conflicts.EmptyScenarios.Slice(), func(id string) DataModelDeleteFieldRef {
				return AdaptDataModelDeleteFieldReportRef(m, id)
			}),
			ScenarioIterations: map[string]*DataModelDeleteFieldConflictIteration{},
			TestRuns:           m.Conflicts.TestRuns,
		},
		ArchivedIterations: pure_utils.Map(m.ArchivedIterations.Slice(), func(id string) DataModelDeleteFieldRef {
			return AdaptDataModelDeleteFieldReportRef(m, id)
		}),
	}

	for iterationId, conflicts := range m.Conflicts.ScenarioIterations {
		r.Conflicts.ScenarioIterations[iterationId] = &DataModelDeleteFieldConflictIteration{
			Name:             conflicts.Name,
			ScenarioId:       conflicts.ScenarioId,
			Draft:            conflicts.Draft,
			TriggerCondition: conflicts.TriggerCondition,
			Rules: pure_utils.Map(conflicts.Rules.Slice(), func(id string) DataModelDeleteFieldRef {
				return AdaptDataModelDeleteFieldReportRef(m, id)
			}),
			Screenings: pure_utils.Map(conflicts.Screening.Slice(), func(id string) DataModelDeleteFieldRef {
				return AdaptDataModelDeleteFieldReportRef(m, id)
			}),
		}
	}

	return r
}

func AdaptDataModelDeleteFieldReportRef(m models.DataModelDeleteFieldReport, id string) DataModelDeleteFieldRef {
	label := id

	if l, ok := m.References[id]; ok {
		label = l
	}

	return DataModelDeleteFieldRef{
		Id:    id,
		Label: label,
	}
}
