package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

func AdaptImportOrgDto(org models.Organization) ImportOrg {
	return ImportOrg{
		Name: org.Name,
		UpdateOrganizationBodyDto: UpdateOrganizationBodyDto{
			DefaultScenarioTimezone: org.DefaultScenarioTimezone,
			SanctionsThreshold:      utils.Ptr(org.OpenSanctionsConfig.MatchThreshold),
			SanctionsLimit:          utils.Ptr(org.OpenSanctionsConfig.MatchLimit),
		},
	}
}

func AdaptImportDataModelDto(
	dataModel models.DataModel,
	links []models.LinkToSingle,
	pivots []models.PivotMetadata,
) ImportDataModel {
	tables := make([]Table, 0, len(dataModel.Tables))
	navigationOptions := make(map[string][]CreateNavigationOptionInput)

	for _, table := range dataModel.Tables {
		// Extract navigation options to top-level map keyed by source table ID
		for _, navOption := range table.NavigationOptions {
			navigationOptions[navOption.SourceTableId] = append(
				navigationOptions[navOption.SourceTableId],
				CreateNavigationOptionInput{
					SourceFieldId:   navOption.SourceFieldId,
					TargetTableId:   navOption.TargetTableId,
					FilterFieldId:   navOption.FilterFieldId,
					OrderingFieldId: navOption.OrderingFieldId,
				},
			)
		}

		var ftmEntity *string
		if table.FTMEntity != nil {
			ftmEntity = utils.Ptr(table.FTMEntity.String())
		}

		// Build table without LinksToSingle and NavigationOptions (those go at top level)
		tables = append(tables, Table{
			ID:           table.ID,
			Name:         table.Name,
			Description:  table.Description,
			Fields:       pure_utils.MapValues(table.Fields, adaptDataModelField),
			FTMEntity:    ftmEntity,
			Alias:        table.Alias,
			SemanticType: table.SemanticType,
			CaptionField: table.CaptionField,
		})
	}

	return ImportDataModel{
		Tables:            tables,
		Links:             pure_utils.Map(links, adaptDataModelLink),
		Pivots:            pure_utils.Map(pivots, AdaptPivotMetadataDto),
		NavigationOptions: navigationOptions,
	}
}

func AdaptImportTagDto(tag models.Tag) ImportTag {
	return ImportTag{
		CreateTagBody: CreateTagBody{
			Name:   tag.Name,
			Color:  tag.Color,
			Target: string(tag.Target),
		},
		Id: tag.Id,
	}
}

func AdaptImportCustomListDto(list models.CustomList, values []models.CustomListValue) ImportCustomList {
	return ImportCustomList{
		CustomList: AdaptCustomListDto(list),
		Values: pure_utils.Map(values, func(v models.CustomListValue) string {
			switch {
			case v.Value != nil:
				return *v.Value
			case v.CidrValue != nil:
				return v.CidrValue.String()
			default:
				return ""
			}
		}),
	}
}

func AdaptImportScenarioDto(scenario models.Scenario, iterationBody ScenarioIterationBodyDto) ImportScenario {
	scenarioDto, _ := AdaptScenarioDto(scenario)
	return ImportScenario{
		Scenario:  scenarioDto,
		Iteration: iterationBody,
	}
}

func AdaptImportWorkflowDto(workflow models.Workflow) ImportWorkflow {
	return ImportWorkflow{
		WorkflowRuleDto: AdaptWorkflowRule(workflow.WorkflowRule),
		ScenarioId:      workflow.ScenarioId,
		Conditions:      pure_utils.Map(workflow.Conditions, AdaptWorkflowCondition),
		Actions:         pure_utils.Map(workflow.Actions, AdaptWorkflowAction),
	}
}
