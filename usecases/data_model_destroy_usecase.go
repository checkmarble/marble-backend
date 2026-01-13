package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/hashicorp/go-set/v2"
)

type DataModelDestroyUsecase struct {
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	enforceSecurity     security.EnforceSecurityOrganization
	dataModelRepository repositories.DataModelRepository

	scenarioRepository          repositories.ScenarioUsecaseRepository
	iterationsRepository        scenarios.ScenarioPublisherRepository
	workflowRepository          workflowRepository
	analyticsSettingsRepository analyticsSettingsRepository
	testRunRepository           repositories.ScenarioTestRunRepository
	clientDbRepository          repositories.OrganizationSchemaRepository
}

func NewDataModelDestroyUsecase(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	enforceSecurity security.EnforceSecurityOrganization,
	dataModelRepository repositories.DataModelRepository,
	scenarioRepository repositories.ScenarioUsecaseRepository,
	iterationsRepository scenarios.ScenarioPublisherRepository,
	workflowRepository workflowRepository,
	analyticsSettingsRepository analyticsSettingsRepository,
	testRunRepository repositories.ScenarioTestRunRepository,
	clientDbRepository repositories.OrganizationSchemaRepository,
) DataModelDestroyUsecase {
	return DataModelDestroyUsecase{
		executorFactory:             executorFactory,
		transactionFactory:          transactionFactory,
		enforceSecurity:             enforceSecurity,
		dataModelRepository:         dataModelRepository,
		scenarioRepository:          scenarioRepository,
		iterationsRepository:        iterationsRepository,
		workflowRepository:          workflowRepository,
		analyticsSettingsRepository: analyticsSettingsRepository,
		testRunRepository:           testRunRepository,
		clientDbRepository:          clientDbRepository,
	}
}

func (uc DataModelDestroyUsecase) RenameField(ctx context.Context, dryRun bool, fieldId string) error {
	return nil
}

func (uc DataModelDestroyUsecase) DeleteTable(ctx context.Context, dryRun bool, tableId string) (models.DataModelDeleteFieldReport, error) {
	exec := uc.executorFactory.NewExecutor()

	table, err := uc.dataModelRepository.GetDataModelTable(ctx, exec, tableId)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if err := uc.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	canDelete, report, err := uc.canDeleteRef(ctx, table.OrganizationID, exec, table, nil)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if !canDelete {
		return report, errors.Wrap(models.ConflictError, "table is used and cannot be deleted")
	}
	if report.ArchivedIterations.Size() > 0 {
		return report, errors.Wrap(models.ConflictError, "table is used and cannot be deleted")
	}
	if dryRun {
		return report, nil
	}

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, table.OrganizationID)
		if err != nil {
			return err
		}

		if err := uc.dataModelRepository.DeleteDataModelTable(ctx, tx, table); err != nil {
			return err
		}

		if err := uc.clientDbRepository.DeleteTable(ctx, clientDbExec, table.Name); err != nil {
			return err
		}

		report.Performed = true

		return nil
	})
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	return report, nil
}

func (uc DataModelDestroyUsecase) DeleteField(ctx context.Context, dryRun bool, fieldId string) (models.DataModelDeleteFieldReport, error) {
	exec := uc.executorFactory.NewExecutor()

	field, err := uc.dataModelRepository.GetDataModelField(ctx, exec, fieldId)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if field.Name == "object_id" || field.Name == "updated_at" {
		return models.DataModelDeleteFieldReport{}, errors.Wrap(models.BadParameterError, "cannot delete internal fields object_id and updated_at")
	}

	table, err := uc.dataModelRepository.GetDataModelTable(ctx, exec, field.TableId)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if err := uc.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	canDelete, report, err := uc.canDeleteRef(ctx, table.OrganizationID, exec, table, &field)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if !canDelete {
		return report, errors.Wrap(models.ConflictError, "field is used and cannot be deleted")
	}
	if dryRun {
		return report, nil
	}

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, table.OrganizationID)
		if err != nil {
			return err
		}

		// If the field was least in at least one iteration, we archive both the field and those iterations
		if report.ArchivedIterations.Size() > 0 {
			for _, it := range report.ArchivedIterations.Slice() {
				if err := uc.iterationsRepository.ArchiveScenarioIteration(ctx, tx, it); err != nil {
					return err
				}
			}

			if err := uc.dataModelRepository.ArchiveDataModelField(ctx, tx, table, field); err != nil {
				return err
			}

			if err := uc.clientDbRepository.RenameField(ctx, clientDbExec, table.Name, field.Name); err != nil {
				return err
			}
		}

		// Otherwise, it was not used anywhere and we can safely delete its metadata as well as the physical column
		if report.ArchivedIterations.Size() == 0 {
			if err := uc.dataModelRepository.DeleteDataModelField(ctx, tx, table, field); err != nil {
				return err
			}

			if err := uc.clientDbRepository.DeleteField(ctx, clientDbExec, table.Name, field.Name); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	report.Performed = true

	return report, nil
}

func (uc DataModelDestroyUsecase) DeleteLink(ctx context.Context, dryRun bool, linkId string) (models.DataModelDeleteFieldReport, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.WriteDataModel(orgId); err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	canDelete, report, err := uc.canDeleteLink(ctx, orgId, exec, linkId)
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if !canDelete {
		return report, errors.Wrap(models.ConflictError, "link is used and cannot be deleted")
	}
	if dryRun {
		return report, nil
	}

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for _, it := range report.ArchivedIterations.Slice() {
			if err := uc.iterationsRepository.ArchiveScenarioIteration(ctx, tx, it); err != nil {
				return err
			}
		}

		if err := uc.dataModelRepository.DeleteDataModelLink(ctx, tx, linkId); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	report.Performed = true

	return report, nil
}

func (uc DataModelDestroyUsecase) DeletePivot(ctx context.Context, dryRun bool, pivotId string) (models.DataModelDeleteFieldReport, error) {
	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.WriteDataModel(orgId); err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	if dryRun {
		return models.NewDataModelDeleteFieldReport(), nil
	}

	if err := uc.dataModelRepository.DeleteDataModelPivot(ctx, exec, pivotId); err != nil {
		return models.DataModelDeleteFieldReport{}, err
	}

	return models.NewDataModelDeleteFieldReport(), nil
}

func (uc DataModelDestroyUsecase) canDeleteRef(
	ctx context.Context,
	orgId string,
	exec repositories.Executor,
	table models.TableMetadata,
	field *models.FieldMetadata,
) (bool, models.DataModelDeleteFieldReport, error) {
	report := models.NewDataModelDeleteFieldReport()
	canDelete := true

	switch field {
	case nil:
		if table.FTMEntity != nil {
			report.Conflicts.ContinuousScreening = true
			canDelete = false
		}

		dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, table.OrganizationID, false, false)
		if err != nil {
			return false, models.DataModelDeleteFieldReport{}, err
		}

		for _, field := range dataModel.Tables[table.Name].Fields {
			if field.FTMProperty != nil {
				report.Conflicts.ContinuousScreening = true
				canDelete = false
			}
		}

	default:
		if field.FTMProperty != nil {
			report.Conflicts.ContinuousScreening = true
			canDelete = false
		}
	}

	analyticsSettings, err := uc.analyticsSettingsRepository.GetAnalyticsSettings(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// Analytics settings can embed fields in exported data.
	for _, setting := range analyticsSettings {
		if setting.TriggerObjectType == table.Name {
			switch field {
			case nil:
				canDelete = false
				report.Conflicts.AnalyticsSettings += 1
			default:
				if slices.Contains(setting.TriggerFields, field.Name) {
					canDelete = false
					report.Conflicts.AnalyticsSettings += 1
				}
			}
		}
	}

	links, err := uc.dataModelRepository.GetLinks(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// If any link starts or ends with the field we want to delete, or,
	// When deleting a table, if any link path contains the table.
	for _, link := range links {
		switch field {
		case nil:
			if link.ParentTableId == table.ID {
				canDelete = false
				report.Conflicts.Links.Insert(link.Id)
			}
			if link.ChildTableId == table.ID {
				canDelete = false
				report.Conflicts.Links.Insert(link.Id)
			}

		default:
			if link.ParentTableId == table.ID && link.ParentFieldName == field.Name {
				canDelete = false
				report.Conflicts.Links.Insert(link.Id)
			}
			if link.ChildTableId == table.ID && link.ChildFieldName == field.Name {
				canDelete = false
				report.Conflicts.Links.Insert(link.Id)
			}
		}

		// For each link that starts with our table, we need to check if there is an
		// analytics setting using it, adding **another field** from that table.
		if link.ParentTableId == table.ID {
			for _, setting := range analyticsSettings {
				tableRef := setting.TriggerObjectType

				for _, dbField := range setting.DbFields {
					if len(dbField.Path) > 0 {
						for _, linkName := range dbField.Path {
							for _, link := range links {
								if link.ChildTableName == tableRef && link.Name == linkName {
									tableRef = link.ParentTableName
								}
							}
						}

						if tableRef == table.Name && (field == nil || dbField.Name == field.Name) {
							canDelete = false
							report.Conflicts.AnalyticsSettings += 1
						}
					}
				}
			}
		}
	}

	pivots, err := uc.dataModelRepository.ListPivots(ctx, exec, orgId, nil, false)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// This only covers table referencing themselves, pivots to other tables are covered by the check on links.
	for _, pivot := range pivots {
		if pivot.BaseTableId == table.ID {
			switch field {
			case nil:
				canDelete = false
			default:
				if pivot.FieldId != nil && *pivot.FieldId == field.ID {
					canDelete = false
					report.Conflicts.Pivots.Insert(pivot.Id.String())
				}
			}
		}
	}

	scenarios, err := uc.scenarioRepository.ListScenariosOfOrganization(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	scenarioMap := make(map[string]models.Scenario)
	for _, s := range scenarios {
		// When deleting a table, we abort if any scenario exists on that trigger object type
		if field == nil && s.TriggerObjectType == table.Name {
			canDelete = false
			report.Conflicts.Scenario.Insert(s.Id)
			report.References[s.Id] = s.Name
		}

		scenarioMap[s.Id] = s
	}

	iterations, err := uc.iterationsRepository.ListAllRulesAndScreenings(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	type scenarioConflict struct {
		hasDraftOrLive        bool
		hasSurvivingIteration bool
	}

	scenarioConflicts := make(map[string]*scenarioConflict)

	// Check all scenario iterations for if the field we want to delete is used in a rule.
	for _, it := range iterations {
		found := false
		scenario := scenarioMap[it.ScenarioId.String()]

		report.References[scenario.Id] = scenario.Name
		report.References[it.ScenarioIterationId.String()] = fmt.Sprintf("%s (%d)", scenario.Name, utils.Or(it.Version, 0))
		report.References[it.RuleId.String()] = it.Name

		if _, ok := scenarioConflicts[scenario.Id]; !ok {
			scenarioConflicts[scenario.Id] = &scenarioConflict{}
		}

		if it.Version == nil || (scenario.LiveVersionID != nil && it.ScenarioIterationId.String() == *scenario.LiveVersionID) {
			scenarioConflicts[scenario.Id].hasDraftOrLive = true
		}

		name := scenario.Name
		if it.Version != nil {
			name = fmt.Sprintf("%s (v%d)", scenario.Name, utils.Or(it.Version, 0))
		}

		iterationReport := models.DataModelDeleteFieldConflictIteration{
			Name:       name,
			ScenarioId: scenario.Id,
			Draft:      it.Version == nil,
			Rules:      set.New[string](0),
			Screening:  set.New[string](0),
		}

		if previousReport, ok := report.Conflicts.ScenarioIterations[it.ScenarioIterationId.String()]; ok {
			iterationReport = *previousReport
		}

		if uc.isRefUsedInAst(it.TriggerAst, scenario.TriggerObjectType, links, table, field) {
			iterationReport.TriggerCondition = true
			found = true
		}

		if it.ScreeningTriggerAst != nil {
			if uc.isRefUsedInAst(it.ScreeningTriggerAst, scenario.TriggerObjectType, links, table, field) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}

		if it.RuleAst != nil {
			if uc.isRefUsedInAst(it.RuleAst, scenario.TriggerObjectType, links, table, field) {
				iterationReport.Rules.Insert(it.RuleId.String())
				found = true
			}
		}
		if it.ScreeningCounterpartyAst != nil {
			if uc.isRefUsedInAst(it.ScreeningCounterpartyAst, scenario.TriggerObjectType, links, table, field) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}
		for _, sc := range it.ScreeningAst {
			if uc.isRefUsedInAst(&sc, scenario.TriggerObjectType, links, table, field) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}

		if found {
			// We cannot delete a field if it is used in a draft or a live scenario iteration
			if it.Version == nil || (scenario.LiveVersionID != nil && it.ScenarioIterationId.String() == *scenario.LiveVersionID) {
				canDelete = false
				report.Conflicts.ScenarioIterations[it.ScenarioIterationId.String()] = &iterationReport
				continue
			}

			// Otherwise, we can delete it but matching iterations will be marked as archived
			report.ArchivedIterations.Insert(it.ScenarioIterationId.String())
		} else {
			scenarioConflicts[scenario.Id].hasSurvivingIteration = true
		}
	}

	for scenarioId, sc := range scenarioConflicts {
		if !sc.hasDraftOrLive && !sc.hasSurvivingIteration {
			canDelete = false
			report.Conflicts.EmptyScenarios.Insert(scenarioId)
		}
	}

	workflows, err := uc.workflowRepository.ListAllOrgWorkflows(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// Workflows can use fields in PayloadEvaluates and case title templates.
	for _, wk := range workflows {
		scenario := scenarioMap[wk.ScenarioId.String()]

		for _, cond := range wk.Conditions {
			switch cond.Function {
			case models.WorkflowPayloadEvaluates:
				var params dto.WorkflowConditionEvaluatesParams
				if err := json.Unmarshal(cond.Params, &params); err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				payloadExpression, err := dto.AdaptASTNode(params.Expression)
				if err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				if uc.isRefUsedInAst(&payloadExpression, scenario.TriggerObjectType, links, table, field) {
					canDelete = false
					report.Conflicts.Workflows.Insert(wk.ScenarioId.String())
				}
			}
		}

		for _, act := range wk.Actions {
			switch act.Action {
			case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
				a, err := models.ParseWorkflowAction[dto.WorkflowActionCaseParams](act)
				if err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				if a.Params.TitleTemplate != nil {
					titleTemplateAst, err := dto.AdaptASTNode(*a.Params.TitleTemplate)
					if err != nil {
						return false, models.DataModelDeleteFieldReport{}, err
					}
					if uc.isRefUsedInAst(&titleTemplateAst, scenario.TriggerObjectType, links, table, field) {
						canDelete = false
						report.Conflicts.Workflows.Insert(wk.ScenarioId.String())
					}
				}
			}
		}
	}

	testRuns, err := uc.testRunRepository.ListRunningTestRun(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// We don't want to delete a table or field used by a running test run
	for _, testRun := range testRuns {
		if report.ArchivedIterations.Contains(testRun.ScenarioIterationId) || report.ArchivedIterations.Contains(testRun.ScenarioLiveIterationId) {
			canDelete = false
			report.Conflicts.TestRuns = true
		}
	}

	return canDelete, report, nil
}

func (uc DataModelDestroyUsecase) canDeleteLink(
	ctx context.Context,
	orgId string,
	exec repositories.Executor,
	linkId string,
) (bool, models.DataModelDeleteFieldReport, error) {
	report := models.NewDataModelDeleteFieldReport()
	canDelete := true

	links, err := uc.dataModelRepository.GetLinks(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	analyticsSettings, err := uc.analyticsSettingsRepository.GetAnalyticsSettings(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	for _, link := range links {
		if link.Id != linkId {
			continue
		}

		for _, setting := range analyticsSettings {
			for _, dbField := range setting.DbFields {
				tableRef := setting.TriggerObjectType

				if len(dbField.Path) > 0 {
					for _, linkName := range dbField.Path {
						for _, link := range links {
							if link.ChildTableName == tableRef {
								if link.Name == linkName {
									canDelete = false
									report.Conflicts.AnalyticsSettings += 1
								}

								tableRef = link.ParentTableName
								break
							}
						}
					}
				}
			}
		}
	}

	pivots, err := uc.dataModelRepository.ListPivots(ctx, exec, orgId, nil, false)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	for _, pivot := range pivots {
		for _, pathLinkId := range pivot.PathLinkIds {
			if pathLinkId == linkId {
				canDelete = false
				report.Conflicts.Pivots.Insert(pivot.Id.String())
			}
		}
	}

	scenarios, err := uc.scenarioRepository.ListScenariosOfOrganization(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	scenarioMap := make(map[string]models.Scenario)
	for _, s := range scenarios {
		scenarioMap[s.Id] = s
	}

	iterations, err := uc.iterationsRepository.ListAllRulesAndScreenings(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	type scenarioConflict struct {
		hasDraftOrLive        bool
		hasSurvivingIteration bool
	}

	scenarioConflicts := make(map[string]*scenarioConflict)

	// Check all scenario iterations for if the link we want to delete is used in a rule.
	for _, it := range iterations {
		found := false
		scenario := scenarioMap[it.ScenarioId.String()]

		report.References[scenario.Id] = scenario.Name
		report.References[it.ScenarioIterationId.String()] = fmt.Sprintf("%s (%d)", scenario.Name, utils.Or(it.Version, 0))
		report.References[it.RuleId.String()] = it.Name

		if _, ok := scenarioConflicts[scenario.Id]; !ok {
			scenarioConflicts[scenario.Id] = &scenarioConflict{}
		}

		if it.Version == nil || (scenario.LiveVersionID != nil && it.ScenarioIterationId.String() == *scenario.LiveVersionID) {
			scenarioConflicts[scenario.Id].hasDraftOrLive = true
		}

		name := scenario.Name
		if it.Version != nil {
			name = fmt.Sprintf("%s (v%d)", scenario.Name, utils.Or(it.Version, 0))
		}

		iterationReport := models.DataModelDeleteFieldConflictIteration{
			Name:       name,
			ScenarioId: scenario.Id,
			Draft:      it.Version == nil,
			Rules:      set.New[string](0),
			Screening:  set.New[string](0),
		}

		if previousReport, ok := report.Conflicts.ScenarioIterations[it.ScenarioIterationId.String()]; ok {
			iterationReport = *previousReport
		}

		if uc.isLinkUsedInAst(it.TriggerAst, links, linkId) {
			iterationReport.TriggerCondition = true
			found = true
		}

		if it.ScreeningTriggerAst != nil {
			if uc.isLinkUsedInAst(it.ScreeningTriggerAst, links, linkId) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}

		if it.RuleAst != nil {
			if uc.isLinkUsedInAst(it.RuleAst, links, linkId) {
				iterationReport.Rules.Insert(it.RuleId.String())
				found = true
			}
		}
		if it.ScreeningCounterpartyAst != nil {
			if uc.isLinkUsedInAst(it.ScreeningCounterpartyAst, links, linkId) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}
		for _, sc := range it.ScreeningAst {
			if uc.isLinkUsedInAst(&sc, links, linkId) {
				iterationReport.Screening.Insert(it.RuleId.String())
				found = true
			}
		}

		if found {
			// We cannot delete a field if it is used in a draft or a live scenario iteration
			if it.Version == nil || (scenario.LiveVersionID != nil && it.ScenarioIterationId.String() == *scenario.LiveVersionID) {
				canDelete = false
				report.Conflicts.ScenarioIterations[it.ScenarioIterationId.String()] = &iterationReport
				continue
			}

			// Otherwise, we can delete it but matching iterations will be marked as archived
			report.ArchivedIterations.Insert(it.ScenarioIterationId.String())
		} else {
			scenarioConflicts[scenario.Id].hasSurvivingIteration = true
		}
	}

	for scenarioId, sc := range scenarioConflicts {
		if !sc.hasDraftOrLive && !sc.hasSurvivingIteration {
			report.Conflicts.EmptyScenarios.Insert(scenarioId)
		}
	}

	workflows, err := uc.workflowRepository.ListAllOrgWorkflows(ctx, exec, orgId)
	if err != nil {
		return false, models.DataModelDeleteFieldReport{}, err
	}

	// Workflows can use fields in PayloadEvaluates and case title templates.
	for _, wk := range workflows {
		for _, cond := range wk.Conditions {
			switch cond.Function {
			case models.WorkflowPayloadEvaluates:
				var params dto.WorkflowConditionEvaluatesParams
				if err := json.Unmarshal(cond.Params, &params); err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				payloadExpression, err := dto.AdaptASTNode(params.Expression)
				if err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				if uc.isLinkUsedInAst(&payloadExpression, links, linkId) {
					canDelete = false
					report.Conflicts.Workflows.Insert(wk.ScenarioId.String())
				}
			}
		}

		for _, act := range wk.Actions {
			switch act.Action {
			case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
				a, err := models.ParseWorkflowAction[dto.WorkflowActionCaseParams](act)
				if err != nil {
					return false, models.DataModelDeleteFieldReport{}, err
				}
				if a.Params.TitleTemplate != nil {
					titleTemplateAst, err := dto.AdaptASTNode(*a.Params.TitleTemplate)
					if err != nil {
						return false, models.DataModelDeleteFieldReport{}, err
					}
					if uc.isLinkUsedInAst(&titleTemplateAst, links, linkId) {
						canDelete = false
						report.Conflicts.Workflows.Insert(wk.ScenarioId.String())
					}
				}
			}
		}
	}

	return canDelete, report, nil
}

func (uc DataModelDestroyUsecase) isRefUsedInAst(tree *ast.Node, triggerObjectType string, links []models.LinkToSingle, table models.TableMetadata, field *models.FieldMetadata) bool {
	if tree == nil {
		return false
	}

	switch tree.Function {
	case ast.FUNC_PAYLOAD:
		if triggerObjectType == table.Name {
			if field != nil {
				if len(tree.Children) > 0 {
					if value, ok := tree.Children[0].Constant.(string); ok && value == field.Name {
						return true
					}
				}
			}
		}

	case ast.FUNC_FILTER:
		tableName, err := tree.ReadConstantNamedChildString("tableName")
		if err != nil {
			return false
		}

		fieldName, err := tree.ReadConstantNamedChildString("fieldName")
		if err != nil {
			return false
		}

		if table.Name == tableName && (field == nil || field.Name == fieldName) {
			return true
		}

		if value, ok := tree.NamedChildren["value"]; ok {
			if found := uc.isRefUsedInAst(utils.Ptr(value), triggerObjectType, links, table, field); found {
				return true
			}
		}

	case ast.FUNC_DB_ACCESS, ast.FUNC_AGGREGATOR:
		if filters, ok := tree.NamedChildren["filters"]; ok {
			if found := uc.isRefUsedInAst(utils.Ptr(filters), triggerObjectType, links, table, field); found {
				return found
			}
		}

		tableRef, err := tree.ReadConstantNamedChildString("tableName")
		if err != nil {
			return false
		}

		pathsToWalk, err := tree.ReadConstantNamedChildStringSlice("path")

		if err == nil {
			for _, path := range pathsToWalk {
				for _, link := range links {
					if link.ChildTableName == tableRef && link.Name == path {
						if field == nil {
							if link.ParentTableName == table.Name || link.ChildTableName == table.Name {
								return true
							}
						}

						tableRef = link.ParentTableName
						break
					}
				}
			}
		}

		if tableRef != table.Name {
			return false
		}

		if field == nil {
			return tableRef == table.Name
		}

		if value, err := tree.ReadConstantNamedChildString("fieldName"); err == nil && value == field.Name {
			return true
		}

	default:
		for _, ch := range tree.Children {
			if found := uc.isRefUsedInAst(&ch, triggerObjectType, links, table, field); found {
				return found
			}
		}
		for _, ch := range tree.NamedChildren {
			if found := uc.isRefUsedInAst(&ch, triggerObjectType, links, table, field); found {
				return found
			}
		}
	}

	return false
}

func (uc DataModelDestroyUsecase) isLinkUsedInAst(tree *ast.Node, links []models.LinkToSingle, linkId string) bool {
	if tree == nil {
		return false
	}

	switch tree.Function {
	case ast.FUNC_FILTER:
		if value, ok := tree.NamedChildren["value"]; ok {
			if found := uc.isLinkUsedInAst(utils.Ptr(value), links, linkId); found {
				return true
			}
		}

	case ast.FUNC_DB_ACCESS, ast.FUNC_AGGREGATOR:
		if filters, ok := tree.NamedChildren["filters"]; ok {
			if found := uc.isLinkUsedInAst(utils.Ptr(filters), links, linkId); found {
				return found
			}
		}

		tableRef, err := tree.ReadConstantNamedChildString("tableName")
		if err != nil {
			return false
		}

		pathsToWalk, err := tree.ReadConstantNamedChildStringSlice("path")
		if err != nil {
			return false
		}

		for _, path := range pathsToWalk {
			for _, link := range links {
				if link.ChildTableName == tableRef && link.Name == path {
					if link.Id == linkId {
						return true
					}

					tableRef = link.ParentTableName
					break
				}
			}
		}

	default:
		for _, ch := range tree.Children {
			if found := uc.isLinkUsedInAst(&ch, links, linkId); found {
				return found
			}
		}
		for _, ch := range tree.NamedChildren {
			if found := uc.isLinkUsedInAst(&ch, links, linkId); found {
				return found
			}
		}
	}

	return false
}
