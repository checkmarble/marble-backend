package usecases

import (
	"context"

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

	orgRepository        repositories.OrganizationRepository
	dataModelRepository  repositories.DataModelRepository
	dataModelUsecase     usecase
	tagRepository        TagUseCaseRepository
	customListRepository repositories.CustomListRepository
	scenarioRepository   repositories.ScenarioUsecaseRepository
	iterationRepository  IterationUsecaseRepository
	inboxRepository      InboxRepository
	workflowRepository   workflowRepository
}

func NewOrgExportUsecase(
	executorFactory executor_factory.ExecutorFactory,
	security security.EnforceSecurityOrgImportImpl,
	orgRepository repositories.OrganizationRepository,
	dataModelRepository repositories.DataModelRepository,
	dataModelUsecase usecase,
	tagRepository TagUseCaseRepository,
	customListRepository repositories.CustomListRepository,
	scenarioRepository repositories.ScenarioUsecaseRepository,
	iterationRepository IterationUsecaseRepository,
	inboxRepository InboxRepository,
	workflowRepository workflowRepository,
) OrgExportUsecase {
	return OrgExportUsecase{
		executorFactory:      executorFactory,
		security:             security,
		orgRepository:        orgRepository,
		dataModelRepository:  dataModelRepository,
		dataModelUsecase:     dataModelUsecase,
		tagRepository:        tagRepository,
		customListRepository: customListRepository,
		scenarioRepository:   scenarioRepository,
		iterationRepository:  iterationRepository,
		inboxRepository:      inboxRepository,
		workflowRepository:   workflowRepository,
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
	pivots := pure_utils.Map(pivotMetadatas, func(p models.PivotMetadata) models.Pivot {
		return p.Enrich(dataModel)
	})

	// Fetch tags (both case and object targets)
	caseTags, err := uc.tagRepository.ListOrganizationTags(ctx, exec, orgId, models.TagTargetCase, false)
	if err != nil {
		return dto.OrgImport{}, errors.Wrap(err, "failed to fetch case tags")
	}
	objectTags, err := uc.tagRepository.ListOrganizationTags(ctx, exec, orgId, models.TagTargetObject, false)
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

	return dto.OrgImport{
		Org:         dto.AdaptImportOrgDto(org),
		DataModel:   dto.AdaptImportDataModelDto(dataModel, links, pivots),
		Tags:        pure_utils.Map(allTags, dto.AdaptImportTagDto),
		CustomLists: importCustomLists,
		Scenarios:   importScenarios,
		Inboxes:     pure_utils.Map(inboxes, dto.AdaptInboxDto),
		Workflows:   pure_utils.Map(workflows, dto.AdaptImportWorkflowDto),
	}, nil
}
