package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type LinkToSingle struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	LinkType        string `json:"link_type"`
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
	ID                string          `json:"id"`
	DataType          string          `json:"data_type"`
	Description       string          `json:"description"`
	Alias             string          `json:"alias"`
	SemanticType      string          `json:"semantic_type,omitempty"`
	IsEnum            bool            `json:"is_enum"`
	Name              string          `json:"name"`
	Nullable          bool            `json:"nullable"`
	TableId           string          `json:"table_id"`
	Values            []any           `json:"values,omitempty"`
	UnicityConstraint string          `json:"unicity_constraint"`
	FTMProperty       *string         `json:"ftm_property,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
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
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name"`
	Description          string                  `json:"description"`
	Fields               map[string]Field        `json:"fields"`
	LinksToSingle        map[string]LinkToSingle `json:"links_to_single,omitempty"`
	NavigationOptions    []NavigationOption      `json:"navigation_options,omitempty"`
	FTMEntity            *string                 `json:"ftm_entity,omitempty"`
	Alias                string                  `json:"alias"`
	SemanticType         models.SemanticType     `json:"semantic_type"`
	CaptionField         string                  `json:"caption_field"`
	PrimaryOrderingField string                  `json:"primary_ordering_field"`
	Metadata             json.RawMessage         `json:"metadata,omitempty"`
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
		Name:                 table.Name,
		ID:                   table.ID,
		Fields:               pure_utils.MapValues(table.Fields, adaptDataModelField),
		LinksToSingle:        pure_utils.MapValues(table.LinksToSingle, adaptDataModelLink),
		NavigationOptions:    pure_utils.Map(table.NavigationOptions, adaptDataModelNavigationOption),
		Description:          table.Description,
		FTMEntity:            ftmEntity,
		Alias:                table.Alias,
		SemanticType:         table.SemanticType,
		CaptionField:         table.CaptionField,
		PrimaryOrderingField: table.PrimaryOrderingField,
		Metadata:             table.Metadata,
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
		Alias:             field.Alias,
		SemanticType:      string(field.SemanticType),
		IsEnum:            field.IsEnum,
		Name:              field.Name,
		Nullable:          field.Nullable,
		TableId:           field.TableId,
		Values:            field.Values,
		UnicityConstraint: field.UnicityConstraint.String(),
		FTMProperty:       ftmProperty,
		Metadata:          field.Metadata,
	}
}

func adaptDataModelLink(linkToSingle models.LinkToSingle) LinkToSingle {
	return LinkToSingle{
		Id:              linkToSingle.Id,
		Name:            linkToSingle.Name,
		LinkType:        string(linkToSingle.LinkType),
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
	Name                 string                 `json:"name"`
	Description          string                 `json:"description"`
	Alias                string                 `json:"alias"`
	SemanticType         string                 `json:"semantic_type"`
	FTMEntity            *string                `json:"ftm_entity"`
	Metadata             json.RawMessage        `json:"metadata"`
	PrimaryOrderingField string                 `json:"primary_ordering_field"`
	Fields               []CreateFieldInput     `json:"fields"`
	Links                []CreateTableLinkInput `json:"links"`
}

func (input CreateTableInput) AdaptToModel() (models.CreateTableInput, error) {
	// Convert DTO fields to model fields
	fields := make([]models.CreateFieldInput, len(input.Fields))
	for i, f := range input.Fields {
		dataType := models.DataTypeFrom(f.Type)
		if dataType == models.UnknownDataType {
			return models.CreateTableInput{}, errors.Wrap(models.BadParameterError, "invalid field data type")
		}
		ftmProperty := (*models.FollowTheMoneyProperty)(nil)
		if f.FTMProperty != nil {
			p := models.FollowTheMoneyPropertyFrom(*f.FTMProperty)
			if p == models.FollowTheMoneyPropertyUnknown {
				return models.CreateTableInput{}, errors.Wrap(
					models.BadParameterError, "invalid FollowTheMoney property")
			}
			ftmProperty = &p
		}
		var fieldSemanticType models.FieldSemanticType
		if f.SemanticType != nil {
			fieldSemanticType = models.FieldSemanticType(*f.SemanticType)
			if !fieldSemanticType.IsValid() {
				return models.CreateTableInput{}, errors.Wrap(
					models.BadParameterError, "invalid field semantic type")
			}
		}
		fields[i] = models.CreateFieldInput{
			Name:         f.Name,
			Description:  f.Description,
			Alias:        f.Alias,
			SemanticType: fieldSemanticType,
			DataType:     dataType,
			Nullable:     f.Nullable,
			IsEnum:       f.IsEnum,
			IsUnique:     f.IsUnique,
			FTMProperty:  ftmProperty,
			Metadata:     f.Metadata,
		}
	}

	// Convert DTO links to model links
	links := make([]models.CreateTableLinkInput, len(input.Links))
	for i, l := range input.Links {
		linkType := models.LinkType(l.LinkType)
		if !linkType.IsValid() {
			return models.CreateTableInput{}, errors.Wrap(models.BadParameterError, "invalid link type")
		}
		links[i] = models.CreateTableLinkInput{
			Name:           l.Name,
			ChildFieldName: l.ChildFieldName,
			ParentTableID:  l.ParentTableId,
			LinkType:       linkType,
		}
	}

	semanticType := models.SemanticTypeFrom(input.SemanticType)
	if !semanticType.IsValid() {
		return models.CreateTableInput{}, errors.Wrap(models.BadParameterError, "invalid semantic type")
	}

	ftmEntity := (*models.FollowTheMoneyEntity)(nil)
	if input.FTMEntity != nil {
		e := models.FollowTheMoneyEntityFrom(*input.FTMEntity)
		if e == models.FollowTheMoneyEntityUnknown {
			return models.CreateTableInput{}, errors.Wrap(models.BadParameterError, "invalid FollowTheMoney entity")
		}
		ftmEntity = &e
	}

	return models.CreateTableInput{
		Name:                 input.Name,
		Description:          input.Description,
		Alias:                input.Alias,
		SemanticType:         semanticType,
		FTMEntity:            ftmEntity,
		Metadata:             input.Metadata,
		Fields:               fields,
		Links:                links,
		PrimaryOrderingField: input.PrimaryOrderingField,
	}, nil
}

// Different than CreateLinkInput because we only have the name of the child field since the field is not created when
// creating the table with fields and links at the same time.
type CreateTableLinkInput struct {
	Name           string `json:"name"`
	LinkType       string `json:"link_type"`
	ChildFieldName string `json:"child_field_name"`
	ParentTableId  string `json:"parent_table_id"`
}

type UpdateTableInput struct {
	Description          *string                 `json:"description"`
	FTMEntity            pure_utils.Null[string] `json:"ftm_entity"`
	Alias                pure_utils.Null[string] `json:"alias"`
	SemanticType         pure_utils.Null[string] `json:"semantic_type"`
	CaptionField         pure_utils.Null[string] `json:"caption_field"`
	PrimaryOrderingField pure_utils.Null[string] `json:"primary_ordering_field"`
	Metadata             *json.RawMessage        `json:"metadata"`
	Fields               []FieldOperation        `json:"fields"`
	Links                []LinkOperation         `json:"links"`
}

func (input UpdateTableInput) AdaptToUpdateTableCompositeInput() (models.UpdateTableCompositeInput, error) {
	result := models.UpdateTableCompositeInput{
		Description: input.Description,
		Metadata:    input.Metadata,
	}

	// Table-level optional fields
	if input.FTMEntity.Set {
		if input.FTMEntity.Valid {
			e := models.FollowTheMoneyEntityFrom(input.FTMEntity.Value())
			if e == models.FollowTheMoneyEntityUnknown {
				return result, errors.Wrap(models.BadParameterError, "invalid FTM entity")
			}
			result.FTMEntity = pure_utils.NullFrom(e)
		} else {
			result.FTMEntity = pure_utils.NullFromPtr[models.FollowTheMoneyEntity](nil)
		}
	}
	if input.Alias.Set {
		result.Alias = input.Alias
	}
	if input.SemanticType.Set {
		if input.SemanticType.Valid {
			t := models.SemanticTypeFrom(input.SemanticType.Value())
			if !t.IsValid() {
				return result, errors.Wrap(models.BadParameterError, "invalid table semantic type")
			}
			result.SemanticType = pure_utils.NullFrom(t)
		} else {
			result.SemanticType = pure_utils.NullFromPtr[models.SemanticType](nil)
		}
	}
	if input.CaptionField.Set {
		result.CaptionField = input.CaptionField
	}
	if input.PrimaryOrderingField.Set {
		result.PrimaryOrderingField = input.PrimaryOrderingField
	}

	// Parse field operations
	for _, fieldOp := range input.Fields {
		switch fieldOp.Op {
		case OpAdd:
			var data CreateFieldInput
			if err := json.Unmarshal(fieldOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid ADD field data: "+err.Error())
			}
			dataType := models.DataTypeFrom(data.Type)
			if dataType == models.UnknownDataType {
				return result, errors.Wrap(models.BadParameterError, "invalid field data type")
			}
			var ftmProperty *models.FollowTheMoneyProperty
			if data.FTMProperty != nil {
				p := models.FollowTheMoneyPropertyFrom(*data.FTMProperty)
				if p == models.FollowTheMoneyPropertyUnknown {
					return result, errors.Wrap(models.BadParameterError, "invalid FollowTheMoney property")
				}
				ftmProperty = &p
			}
			var fieldSemanticType models.FieldSemanticType
			if data.SemanticType != nil {
				fieldSemanticType = models.FieldSemanticType(*data.SemanticType)
				if !fieldSemanticType.IsValid() {
					return result, errors.Wrap(models.BadParameterError, "invalid field semantic type")
				}
			}
			result.FieldsToAdd = append(result.FieldsToAdd, models.CreateFieldInput{
				Name:         data.Name,
				Description:  data.Description,
				Alias:        data.Alias,
				SemanticType: fieldSemanticType,
				DataType:     dataType,
				Nullable:     data.Nullable,
				IsEnum:       data.IsEnum,
				IsUnique:     data.IsUnique,
				FTMProperty:  ftmProperty,
				Metadata:     data.Metadata,
			})

		case OpMod:
			var data ModFieldOperationData
			if err := json.Unmarshal(fieldOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid MOD field data: "+err.Error())
			}
			if data.ID == "" {
				return result, errors.Wrap(models.BadParameterError, "MOD field operation requires an id")
			}
			updateInput := models.UpdateFieldInput{
				Description: data.Description,
				IsEnum:      data.IsEnum,
				IsUnique:    data.IsUnique,
				IsNullable:  data.IsNullable,
				Alias:       data.Alias,
				Metadata:    data.Metadata,
			}
			if data.FTMProperty.Set {
				if data.FTMProperty.Valid {
					p := models.FollowTheMoneyPropertyFrom(data.FTMProperty.Value())
					if p == models.FollowTheMoneyPropertyUnknown {
						return result, errors.Wrap(models.BadParameterError, "invalid FTM property")
					}
					updateInput.FTMProperty = pure_utils.NullFrom(p)
				} else {
					updateInput.FTMProperty = pure_utils.NullFromPtr[models.FollowTheMoneyProperty](nil)
				}
			}
			if data.SemanticType.Set {
				if data.SemanticType.Valid {
					st := models.FieldSemanticType(data.SemanticType.Value())
					if !st.IsValid() {
						return result, errors.Wrap(models.BadParameterError, "invalid field semantic type")
					}
					updateInput.SemanticType = pure_utils.NullFrom(st)
				} else {
					updateInput.SemanticType = pure_utils.NullFromPtr[models.FieldSemanticType](nil)
				}
			}
			result.FieldsToUpdate = append(result.FieldsToUpdate, models.UpdateFieldWithID{
				ID:               data.ID,
				UpdateFieldInput: updateInput,
			})

		case OpDel:
			var data DeleteOperationData
			if err := json.Unmarshal(fieldOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid DEL field data: "+err.Error())
			}
			if data.ID == "" {
				return result, errors.Wrap(models.BadParameterError, "DEL field operation requires an id")
			}
			result.FieldsToDelete = append(result.FieldsToDelete, data.ID)

		default:
			return result, errors.Wrapf(models.BadParameterError,
				"invalid field operation: %s", fieldOp.Op)
		}
	}

	// Parse link operations
	for _, linkOp := range input.Links {
		switch linkOp.Op {
		case OpAdd:
			var data CreateTableLinkInput
			if err := json.Unmarshal(linkOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid ADD link data: "+err.Error())
			}
			linkType := models.LinkType(data.LinkType)
			if !linkType.IsValid() {
				return result, errors.Wrap(models.BadParameterError, "invalid link type")
			}
			result.LinksToAdd = append(result.LinksToAdd, models.CreateTableLinkInput{
				Name:           data.Name,
				LinkType:       linkType,
				ChildFieldName: data.ChildFieldName,
				ParentTableID:  data.ParentTableId,
			})

		case OpMod:
			var data ModLinkOperationData
			if err := json.Unmarshal(linkOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid MOD link data: "+err.Error())
			}
			if data.ID == "" {
				return result, errors.Wrap(models.BadParameterError, "MOD link operation requires an id")
			}
			linkType := models.LinkType(data.LinkType)
			if !linkType.IsValid() {
				return result, errors.Wrap(models.BadParameterError, "invalid link type")
			}
			result.LinksToUpdate = append(result.LinksToUpdate, models.UpdateLinkWithID{
				ID:       data.ID,
				LinkType: linkType,
			})

		case OpDel:
			var data DeleteOperationData
			if err := json.Unmarshal(linkOp.Data, &data); err != nil {
				return result, errors.Wrap(models.BadParameterError, "invalid DEL link data: "+err.Error())
			}
			if data.ID == "" {
				return result, errors.Wrap(models.BadParameterError, "DEL link operation requires an id")
			}
			result.LinksToDelete = append(result.LinksToDelete, data.ID)

		default:
			return result, errors.Wrapf(models.BadParameterError,
				"invalid link operation: %s", linkOp.Op)
		}
	}

	return result, nil
}

type UpdateTableOperationType string

const (
	OpAdd UpdateTableOperationType = "ADD"
	OpMod UpdateTableOperationType = "MOD"
	OpDel UpdateTableOperationType = "DEL"
)

type FieldOperation struct {
	Op   UpdateTableOperationType `json:"op"`
	Data json.RawMessage          `json:"data"`
}

type LinkOperation struct {
	Op   UpdateTableOperationType `json:"op"`
	Data json.RawMessage          `json:"data"`
}

type DeleteOperationData struct {
	ID string `json:"id"`
}

type ModFieldOperationData struct {
	ID           string                  `json:"id"`
	Description  *string                 `json:"description"`
	IsEnum       *bool                   `json:"is_enum"`
	IsUnique     *bool                   `json:"is_unique"`
	IsNullable   *bool                   `json:"is_nullable"`
	FTMProperty  pure_utils.Null[string] `json:"ftm_property"`
	Alias        *string                 `json:"alias"`
	SemanticType pure_utils.Null[string] `json:"semantic_type"`
	Metadata     *json.RawMessage        `json:"metadata"`
}

type ModLinkOperationData struct {
	ID       string `json:"id"`
	LinkType string `json:"link_type"`
}

// Create link input outside the context of creating a table.
type CreateLinkInput struct {
	Name          string `json:"name"`
	LinkType      string `json:"link_type"`
	ParentTableId string `json:"parent_table_id"`
	ParentFieldId string `json:"parent_field_id"`
	ChildTableId  string `json:"child_table_id"`
	ChildFieldId  string `json:"child_field_id"`
}

func (input CreateLinkInput) AdaptToModel(organizationID uuid.UUID) (models.DataModelLinkCreateInput, error) {
	linkType := models.LinkType(input.LinkType)
	if !linkType.IsValid() {
		return models.DataModelLinkCreateInput{},
			errors.Wrap(models.BadParameterError, "invalid link type")
	}

	return models.DataModelLinkCreateInput{
		OrganizationID: organizationID,
		Name:           input.Name,
		LinkType:       linkType,
		ParentTableID:  input.ParentTableId,
		ParentFieldID:  input.ParentFieldId,
		ChildTableID:   input.ChildTableId,
		ChildFieldID:   input.ChildFieldId,
	}, nil
}

type UpdateFieldInput struct {
	Description  *string                 `json:"description"`
	IsEnum       *bool                   `json:"is_enum"`
	IsUnique     *bool                   `json:"is_unique"`
	IsNullable   *bool                   `json:"is_nullable"`
	FTMProperty  pure_utils.Null[string] `json:"ftm_property"`
	Alias        *string                 `json:"alias"`
	SemanticType pure_utils.Null[string] `json:"semantic_type"`
	Metadata     *json.RawMessage        `json:"metadata"`
}

type CreateFieldInput struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Type         string          `json:"type"`
	Alias        string          `json:"alias"`
	SemanticType *string         `json:"semantic_type"`
	Nullable     bool            `json:"nullable"`
	IsEnum       bool            `json:"is_enum"`
	IsUnique     bool            `json:"is_unique"`
	FTMProperty  *string         `json:"ftm_property"`
	Metadata     json.RawMessage `json:"metadata"`
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
	Error              string                        `json:"error,omitempty"`
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
	TestRuns             bool                                              `json:"test_runs"`
	PrimaryOrderingField bool                                              `json:"primary_ordering_field"`
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

func AdaptDataModelDeleteFieldReport(m models.DataModelDeleteFieldReport, errs ...error) DataModelDeleteFieldReport {
	var errMsg string
	for _, err := range errs {
		if err != nil {
			errMsg = err.Error()
			break
		}
	}
	r := DataModelDeleteFieldReport{
		Error:     errMsg,
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
			TestRuns:             m.Conflicts.TestRuns,
			PrimaryOrderingField: m.Conflicts.PrimaryOrderingField,
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
