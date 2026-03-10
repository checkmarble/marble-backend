package usecases

import (
	"context"
	"encoding/json"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type OrgExportUsecase struct {
	executorFactory executor_factory.ExecutorFactory
	security        security.EnforceSecurityOrgImportImpl
	apiVersion      string

	orgRepository             repositories.OrganizationRepository
	dataModelRepository       repositories.DataModelRepository
	dataModelUsecase          usecase
	tagRepository             TagUseCaseRepository
	customListRepository      repositories.CustomListRepository
	scenarioRepository        repositories.ScenarioUsecaseRepository
	iterationRepository       IterationUsecaseRepository
	screeningConfigRepository ScreeningConfigRepository
	inboxRepository           InboxRepository
	workflowRepository        workflowRepository
}

func NewOrgExportUsecase(
	executorFactory executor_factory.ExecutorFactory,
	security security.EnforceSecurityOrgImportImpl,
	apiVersion string,
	orgRepository repositories.OrganizationRepository,
	dataModelRepository repositories.DataModelRepository,
	dataModelUsecase usecase,
	tagRepository TagUseCaseRepository,
	customListRepository repositories.CustomListRepository,
	scenarioRepository repositories.ScenarioUsecaseRepository,
	iterationRepository IterationUsecaseRepository,
	screeningConfigRepository ScreeningConfigRepository,
	inboxRepository InboxRepository,
	workflowRepository workflowRepository,
) OrgExportUsecase {
	return OrgExportUsecase{
		executorFactory:           executorFactory,
		security:                  security,
		apiVersion:                apiVersion,
		orgRepository:             orgRepository,
		dataModelRepository:       dataModelRepository,
		dataModelUsecase:          dataModelUsecase,
		tagRepository:             tagRepository,
		customListRepository:      customListRepository,
		scenarioRepository:        scenarioRepository,
		iterationRepository:       iterationRepository,
		screeningConfigRepository: screeningConfigRepository,
		inboxRepository:           inboxRepository,
		workflowRepository:        workflowRepository,
	}
}

func (uc *OrgExportUsecase) Export(ctx context.Context, orgId uuid.UUID) (dto.OrgImport, error) {
	logger := utils.LoggerFromContext(ctx)

	if err := uc.security.ExportOrg(orgId); err != nil {
		return dto.OrgImport{}, err
	}

	exec := uc.executorFactory.NewExecutor()

	// Fetch organization
	org, err := uc.orgRepository.GetOrganizationById(ctx, exec, orgId)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch organization")
	}

	// Fetch data model with navigation options and unicity constraints
	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeNavigationOptions:  true,
		IncludeUnicityConstraints: true,
	}, false)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch data model")
	}

	// Fetch links
	links, err := uc.dataModelRepository.GetLinks(ctx, exec, orgId)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch links")
	}

	// Fetch pivots and enrich them
	pivotMetadatas, err := uc.dataModelRepository.ListPivots(ctx, exec, orgId, nil, false)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch pivots")
	}

	// Fetch tags (both case and object targets)
	caseTags, err := uc.tagRepository.ListOrganizationTags(ctx, exec, orgId, models.TagTargetCase, false, nil)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch case tags")
	}
	objectTags, err := uc.tagRepository.ListOrganizationTags(ctx, exec, orgId, models.TagTargetObject, false, nil)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch object tags")
	}
	allTags := append(caseTags, objectTags...)

	// Fetch custom lists with values
	lists, err := uc.customListRepository.AllCustomLists(ctx, exec, orgId)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch custom lists")
	}
	importCustomLists := make([]dto.ImportCustomList, 0, len(lists))
	for _, list := range lists {
		values, err := uc.customListRepository.GetCustomListValues(ctx, exec,
			models.GetCustomListValuesInput{Id: list.Id})
		if err != nil {
			return dto.OrgImport{}, errors.Wrapf(err,
				"failed to fetch custom list values for %s", list.Id)
		}
		importCustomLists = append(importCustomLists,
			dto.AdaptImportCustomListDto(list, values))
	}

	// Fetch scenarios with published iterations
	scenarios, err := uc.scenarioRepository.ListScenariosOfOrganization(ctx, exec, orgId)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch scenarios")
	}
	importScenarios := make([]dto.ImportScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		if scenario.LiveVersionID == nil {
			logger.DebugContext(ctx, "skipping scenario without published iteration",
				"scenario_id", scenario.Id, "scenario_name", scenario.Name)
			continue
		}

		iteration, err := uc.iterationRepository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID, false)
		if err != nil {
			return dto.OrgImport{}, errors.Wrapf(err,
				"failed to fetch iteration for scenario %s", scenario.Id)
		}

		iteration.ScreeningConfigs, err = uc.screeningConfigRepository.ListScreeningConfigs(
			ctx, exec, iteration.Id, false)
		if err != nil {
			return dto.OrgImport{}, errors.Wrapf(err,
				"failed to fetch screening configs for iteration %s", iteration.Id)
		}

		iterationWithBody, err := dto.AdaptScenarioIterationWithBodyDto(iteration)
		if err != nil {
			return dto.OrgImport{}, errors.Wrapf(err,
				"failed to adapt iteration for scenario %s", scenario.Id)
		}

		importScenarios = append(importScenarios,
			dto.AdaptImportScenarioDto(scenario, iterationWithBody.Body))
	}

	// Fetch inboxes
	inboxes, err := uc.inboxRepository.ListInboxes(ctx, exec, orgId, nil, false)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch inboxes")
	}
	for i := range inboxes {
		inboxes[i].InboxUsers = nil // Don't need to export users
	}

	// Fetch workflows
	workflows, err := uc.workflowRepository.ListAllOrgWorkflows(ctx, exec, orgId)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch workflows")
	}
	// Filter out workflows with scenarios that don't have a published iteration (since we won't be able to export those scenarios)
	filteredWorkflows := make([]models.Workflow, 0, len(workflows))
	exportedScenarioIds := make(map[uuid.UUID]struct{}, len(importScenarios))
	for i := range importScenarios {
		exportedScenarioIds[uuid.MustParse(importScenarios[i].Scenario.Id)] = struct{}{}
	}
	// Build set of all exported rule stable IDs (rules + screening configs), to filter out workflow conditions referencing removed rules
	exportedRuleStableIds := make(map[string]struct{})
	for _, importScenario := range importScenarios {
		for _, rule := range importScenario.Iteration.Rules {
			exportedRuleStableIds[rule.StableId] = struct{}{}
		}
		for _, scc := range importScenario.Iteration.ScreeningConfigs {
			exportedRuleStableIds[scc.StableId] = struct{}{}
		}
	}
	for _, workflow := range workflows {
		if _, ok := exportedScenarioIds[workflow.ScenarioId]; !ok {
			continue
		}

		cleanedConditions, valid := filterWorkflowConditions(ctx, workflow, exportedRuleStableIds)
		if !valid {
			continue
		}

		workflow.Conditions = cleanedConditions
		filteredWorkflows = append(filteredWorkflows, workflow)
	}

	return dto.OrgImport{
		Metadata: dto.OrgImportMetadata{
			AppVersion: uc.apiVersion,
		},
		Org:         dto.AdaptImportOrgDto(org),
		DataModel:   dto.AdaptImportDataModelDto(dataModel, links, pivotMetadatas),
		Tags:        pure_utils.Map(allTags, dto.AdaptImportTagDto),
		CustomLists: importCustomLists,
		Scenarios:   importScenarios,
		Inboxes:     pure_utils.Map(inboxes, dto.AdaptInboxDto),
		Workflows:   pure_utils.Map(filteredWorkflows, dto.AdaptImportWorkflowDto),
	}, nil
}

// filterWorkflowConditions cleans rule_hit conditions by removing rule stable IDs that are
// not present in the exported scenarios. Returns the cleaned conditions and whether the
// workflow should be included in the export.
func filterWorkflowConditions(
	ctx context.Context,
	workflow models.Workflow,
	exportedRuleStableIds map[string]struct{},
) ([]models.WorkflowCondition, bool) {
	logger := utils.LoggerFromContext(ctx)
	cleaned := make([]models.WorkflowCondition, 0, len(workflow.Conditions))

	for _, cond := range workflow.Conditions {
		if cond.Function != models.WorkflowConditionRuleHit {
			cleaned = append(cleaned, cond)
			continue
		}

		var p dto.WorkflowConditionRuleHitParams
		if err := json.Unmarshal(cond.Params, &p); err != nil {
			logger.InfoContext(ctx, "skipping workflow export: could not parse rule_hit condition params",
				"workflow_id", workflow.Id, "condition_id", cond.Id)
			return nil, false
		}

		validIds := make([]uuid.UUID, 0, len(p.RuleId))
		for _, ruleId := range p.RuleId {
			if _, ok := exportedRuleStableIds[ruleId.String()]; ok {
				validIds = append(validIds, ruleId)
			}
		}

		if len(validIds) == 0 {
			logger.InfoContext(ctx, "skipping workflow export: rule_hit condition references only rules not present in the live iteration",
				"workflow_id", workflow.Id, "condition_id", cond.Id)
			return nil, false
		}

		if len(validIds) < len(p.RuleId) {
			logger.InfoContext(ctx, "removed some rule_id references from rule_hit condition during export: rules not present in live iteration",
				"workflow_id", workflow.Id, "condition_id", cond.Id,
				"original_count", len(p.RuleId), "valid_count", len(validIds))
			updatedParams, err := json.Marshal(dto.WorkflowConditionRuleHitParams{RuleId: validIds})
			if err != nil {
				logger.InfoContext(ctx, "skipping workflow export: could not re-marshal rule_hit condition params",
					"workflow_id", workflow.Id, "condition_id", cond.Id)
				return nil, false
			}
			cond.Params = updatedParams
		}

		cleaned = append(cleaned, cond)
	}

	return cleaned, true
}
