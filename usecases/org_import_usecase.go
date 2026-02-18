package usecases

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type OrgImportUsecase struct {
	transactionWrapper UsecaseTransactionWrapper
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	security           security.EnforceSecurityOrgImportImpl

	orgRepository        repositories.OrganizationRepository
	schemaRepository     repositories.OrganizationSchemaRepository
	userRepository       repositories.UserRepository
	firebaseAdminer      idp.Adminer
	dataModelRepository  repositories.DataModelRepository
	dataModelUsecase     usecase
	tagRepository        TagUseCaseRepository
	customListRepository repositories.CustomListRepository
	scenarioRepository   repositories.ScenarioUsecaseRepository
	iterationRepository  IterationUsecaseRepository
	screeningRepository  ScreeningConfigRepository
	indexEditor          indexes.ClientDbIndexEditor
	publicationUsecase   *ScenarioPublicationUsecase
	inboxRepository      InboxRepository
	workflowRepository   workflowRepository

	ingestionUsecase IngestionUseCase
	decisionUsecase  DecisionUsecase
}

func NewOrgImportUsecase(
	wrapper UsecaseTransactionWrapper,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	security security.EnforceSecurityOrgImportImpl,
	organizationRepository repositories.OrganizationRepository,
	schemaRepository repositories.OrganizationSchemaRepository,
	userRepository repositories.UserRepository,
	firebaseAdminer idp.Adminer,
	dataModelRepository repositories.DataModelRepository,
	dataModelUsecase usecase,
	tagRepository TagUseCaseRepository,
	customListRepository repositories.CustomListRepository,
	scenarioRepository repositories.ScenarioUsecaseRepository,
	iterationRepository IterationUsecaseRepository,
	screeningRepository ScreeningConfigRepository,
	indexEditor indexes.ClientDbIndexEditor,
	publicationUsecase *ScenarioPublicationUsecase,
	inboxRepository InboxRepository,
	workflowRepository workflowRepository,
	ingestionUsecase IngestionUseCase,
	decisionUsecase DecisionUsecase,
) OrgImportUsecase {
	return OrgImportUsecase{
		transactionWrapper:   wrapper,
		executorFactory:      executorFactory,
		transactionFactory:   transactionFactory,
		security:             security,
		orgRepository:        organizationRepository,
		schemaRepository:     schemaRepository,
		userRepository:       userRepository,
		firebaseAdminer:      firebaseAdminer,
		dataModelRepository:  dataModelRepository,
		dataModelUsecase:     dataModelUsecase,
		tagRepository:        tagRepository,
		customListRepository: customListRepository,
		scenarioRepository:   scenarioRepository,
		iterationRepository:  iterationRepository,
		screeningRepository:  screeningRepository,
		indexEditor:          indexEditor,
		publicationUsecase:   publicationUsecase,
		inboxRepository:      inboxRepository,
		workflowRepository:   workflowRepository,
		ingestionUsecase:     ingestionUsecase,
		decisionUsecase:      decisionUsecase,
	}
}

//go:embed archetypes/*.json
var ARCHETYPES embed.FS

func (uc OrgImportUsecase) ListArchetypes(ctx context.Context) ([]models.ArchetypeInfo, error) {
	if err := uc.security.ListOrgArchetypes(); err != nil {
		return nil, err
	}

	entries, err := ARCHETYPES.ReadDir("archetypes")
	if err != nil {
		return nil, err
	}

	archetypes := make([]models.ArchetypeInfo, len(entries))
	for i, entry := range entries {
		filename := entry.Name()

		d, err := ARCHETYPES.ReadFile(fmt.Sprintf("archetypes/%s", filename))
		if err != nil {
			return nil, err
		}

		var spec dto.OrgImportMetadata
		if err := json.Unmarshal(d, &spec); err != nil {
			return nil, err
		}

		archetypes[i] = models.ArchetypeInfo{
			Name:        filename[:len(filename)-len(".json")],
			Label:       spec.Label,
			Description: spec.Description,
		}
	}

	return archetypes, nil
}

func (uc *OrgImportUsecase) ImportFromArchetype(ctx context.Context, archetype string, spec dto.OrgImport, seed bool) (uuid.UUID, error) {
	d, err := ARCHETYPES.ReadFile(fmt.Sprintf("archetypes/%s.json", archetype))
	if err != nil {
		return uuid.Nil, err
	}

	var pattern dto.OrgImport

	if err := json.Unmarshal(d, &pattern); err != nil {
		return uuid.Nil, err
	}

	pattern.Org.Name = spec.Org.Name
	pattern.Admins = spec.Admins

	return uc.Import(ctx, pattern, seed)
}

func (uc *OrgImportUsecase) Import(ctx context.Context, spec dto.OrgImport, seed bool) (uuid.UUID, error) {
	if err := uc.security.ImportOrg(); err != nil {
		return uuid.Nil, err
	}

	return executor_factory.TransactionReturnValue(ctx, uc.transactionFactory, func(
		tx repositories.Transaction,
	) (uuid.UUID, error) {
		orgId, err := uc.createOrganization(ctx, tx, spec)
		if err != nil {
			return orgId, err
		}

		if seed {
			if err := uc.Seed(ctx, spec, orgId); err != nil {
				return orgId, nil
			}
		}

		return orgId, nil
	})
}

func (uc *OrgImportUsecase) createOrganization(ctx context.Context, tx repositories.Transaction, spec dto.OrgImport) (uuid.UUID, error) {
	ids := make(map[string]string)
	orgId, _ := uuid.NewV7()

	if err := uc.orgRepository.CreateOrganization(ctx, tx, orgId, models.CreateOrganizationInput{
		Name: spec.Org.Name,
	}); err != nil {
		return uuid.Nil, err
	}

	org, err := uc.orgRepository.GetOrganizationById(ctx, tx, orgId)
	if err != nil {
		return uuid.Nil, err
	}

	admins, err := uc.createAdmins(ctx, tx, orgId, spec.Admins)
	if err != nil {
		return uuid.Nil, err
	}

	admin, err := uc.userRepository.UserById(ctx, tx, admins[0])
	if err != nil {
		return uuid.Nil, err
	}

	*uc = uc.transactionWrapper(tx, org, admin).NewOrgImportUsecase()

	err = uc.orgRepository.UpdateOrganization(ctx, tx, orgId, models.UpdateOrganizationInput{
		DefaultScenarioTimezone: spec.Org.UpdateOrganizationBodyDto.DefaultScenarioTimezone,
		ScreeningConfig: models.OrganizationOpenSanctionsConfigUpdateInput{
			MatchThreshold: spec.Org.SanctionsThreshold,
			MatchLimit:     spec.Org.SanctionsLimit,
		},
	})
	if err != nil {
		return uuid.Nil, err
	}

	if err := uc.createDataModel(ctx, tx, orgId, ids, spec.DataModel); err != nil {
		return uuid.Nil, err
	}
	if err := uc.createTags(ctx, tx, orgId, ids, spec.Tags); err != nil {
		return uuid.Nil, err
	}
	if err := uc.createCustomLists(ctx, tx, orgId, ids, spec.CustomLists); err != nil {
		return uuid.Nil, err
	}
	if err := uc.createScenarios(ctx, tx, orgId, ids, spec.Scenarios); err != nil {
		return uuid.Nil, err
	}
	if err := uc.createInboxes(ctx, tx, orgId, ids, spec.Inboxes); err != nil {
		return uuid.Nil, err
	}
	if err := uc.createWorkflows(ctx, tx, ids, spec.Workflows); err != nil {
		return uuid.Nil, err
	}

	return orgId, nil
}

func (uc *OrgImportUsecase) createAdmins(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, admins []dto.CreateUser,
) ([]string, error) {
	users := make([]string, len(admins))

	for idx, admin := range admins {
		userId, err := uc.userRepository.CreateUser(ctx, tx, models.CreateUser{
			OrganizationId: orgId,
			Email:          admin.Email,
			FirstName:      admin.FirstName,
			LastName:       admin.LastName,
			Role:           models.ADMIN,
		})
		if err != nil {
			return nil, err
		}

		if uc.firebaseAdminer != nil {
			if err := uc.firebaseAdminer.CreateUser(ctx, admin.Email,
				fmt.Sprintf("%s %s", admin.FirstName, admin.LastName)); err != nil {
				return nil, err
			}
		}

		users[idx] = userId
	}

	return users, nil
}

func (uc *OrgImportUsecase) createDataModel(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, ids map[string]string, dataModel dto.ImportDataModel,
) error {
	clientDbExec, err := uc.executorFactory.NewClientDbExecutor(ctx, orgId)
	if err != nil {
		return err
	}

	if err := uc.schemaRepository.CreateSchemaIfNotExists(ctx, clientDbExec); err != nil {
		return err
	}

	for _, table := range dataModel.Tables {
		tableId, _ := uuid.NewV7()
		ids[table.ID] = tableId.String()

		if err := uc.schemaRepository.CreateTable(ctx, clientDbExec, table.Name); err != nil {
			return err
		}

		if err := uc.dataModelRepository.CreateDataModelTable(ctx, tx, orgId,
			tableId.String(), table.Name, table.Description /* TODO */, nil); err != nil {
			return err
		}

		for name, field := range table.Fields {
			fieldId, _ := uuid.NewV7()
			ids[field.ID] = fieldId.String()

			field := models.CreateFieldInput{
				TableId:     tableId.String(),
				Name:        name,
				Description: field.Description,
				DataType:    models.DataTypeFrom(field.DataType),
				Nullable:    field.Nullable,
				IsEnum:      field.IsEnum,
				IsUnique:    field.UnicityConstraint != "",
				FTMProperty: nil, /* TODO */
			}

			if err := uc.schemaRepository.CreateField(ctx, clientDbExec, table.Name, field); err != nil {
				return err
			}

			if err := uc.dataModelRepository.CreateDataModelField(ctx, tx, orgId, fieldId.String(), field); err != nil {
				return err
			}
		}
	}

	for _, link := range dataModel.Links {
		linkId, _ := uuid.NewV7()
		ids[link.Id] = linkId.String()

		err := uc.dataModelRepository.CreateDataModelLink(ctx, tx, linkId.String(), models.DataModelLinkCreateInput{
			OrganizationID: orgId,
			Name:           fmt.Sprintf("%s_%s", link.ChildTableName, link.ParentTableName),
			ParentTableID:  ids[link.ParentTableId],
			ParentFieldID:  ids[link.ParentFieldId],
			ChildTableID:   ids[link.ChildTableId],
			ChildFieldID:   ids[link.ChildFieldId],
		})
		if err != nil {
			return err
		}
	}

	for _, pivot := range dataModel.Pivots {
		pivotId, _ := uuid.NewV7()
		ids[pivot.Id.String()] = pivotId.String()

		var field *string
		if pivot.FieldId != "" {
			field = utils.Ptr(ids[pivot.FieldId])
		}

		err := uc.dataModelRepository.CreatePivot(ctx, tx, pivot.Id.String(), models.CreatePivotInput{
			OrganizationId: orgId,
			BaseTableId:    ids[pivot.BaseTableId],
			FieldId:        field,
			PathLinkIds: pure_utils.Map(pivot.PathLinkIds, func(id string) string {
				return ids[id]
			}),
		})
		if err != nil {
			return err
		}
	}

	for navTable, navOptions := range dataModel.NavigationOptions {
		err := uc.dataModelUsecase.CreateNavigationOption(ctx, models.CreateNavigationOptionInput{
			Blocking:        true,
			SourceTableId:   ids[navTable],
			SourceFieldId:   ids[navOptions.SourceFieldId],
			TargetTableId:   ids[navOptions.TargetTableId],
			FilterFieldId:   ids[navOptions.FilterFieldId],
			OrderingFieldId: ids[navOptions.OrderingFieldId],
		})
		if err != nil {
			// Navigation options are checked for duplication, we want to ignore that
			if !errors.Is(err, models.ConflictError) {
				return err
			}
		}
	}

	return nil
}

func (uc *OrgImportUsecase) createTags(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, ids map[string]string, tags []dto.ImportTag,
) error {
	for _, tag := range tags {
		tagId, _ := uuid.NewV7()
		ids[tag.Id] = tagId.String()

		err := uc.tagRepository.CreateTag(ctx, tx, models.CreateTagAttributes{
			OrganizationId: orgId,
			Name:           tag.Name,
			Color:          tag.Color,
			Target:         models.TagTarget(tag.Target),
		}, tagId.String())
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc OrgImportUsecase) createCustomLists(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, ids map[string]string, lists []dto.ImportCustomList,
) error {
	for _, list := range lists {
		listId, _ := uuid.NewV7()
		ids[list.Id] = listId.String()

		err := uc.customListRepository.CreateCustomList(ctx, tx, models.CreateCustomListInput{
			OrganizationId: orgId,
			Name:           list.Name,
			Description:    list.Description,
		}, listId.String())
		if err != nil {
			return err
		}

		err = uc.customListRepository.BatchInsertCustomListValues(ctx, tx, listId.String(), pure_utils.Map(
			list.Values, func(v string) models.BatchInsertCustomListValue {
				valueId, _ := uuid.NewV7()
				return models.BatchInsertCustomListValue{Id: valueId.String(), Value: v}
			}), nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc *OrgImportUsecase) createScenarios(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, ids map[string]string, scenarios []dto.ImportScenario,
) error {
	for _, scenario := range scenarios {
		scenarioId, _ := uuid.NewV7()
		ids[scenario.Scenario.Id] = scenarioId.String()

		err := uc.scenarioRepository.CreateScenario(ctx, tx, orgId, models.CreateScenarioInput{
			OrganizationId:    orgId,
			Name:              scenario.Scenario.Name,
			Description:       scenario.Scenario.Description,
			TriggerObjectType: scenario.Scenario.TriggerObjectType,
		}, scenarioId.String())
		if err != nil {
			return err
		}

		var triggerCondition *ast.Node
		if scenario.Iteration.TriggerConditionAstExpression != nil {
			tc, err := dto.AdaptASTNode(*scenario.Iteration.TriggerConditionAstExpression)
			if err != nil {
				return err
			}

			triggerCondition = &tc

			if err := uc.adaptAstNodeIds(ctx, ids, &tc); err != nil {
				return err
			}
		}

		rules := make([]models.CreateRuleInput, len(scenario.Iteration.Rules))

		for idx, rule := range scenario.Iteration.Rules {
			stableId, _ := uuid.NewV7()
			ids[rule.StableId] = stableId.String()

			var ruleAst *ast.Node

			if rule.FormulaAstExpression != nil {
				r, err := dto.AdaptASTNode(*rule.FormulaAstExpression)
				if err != nil {
					return err
				}

				ruleAst = &r

				if err := uc.adaptAstNodeIds(ctx, ids, &r); err != nil {
					return err
				}
			}

			rules[idx] = models.CreateRuleInput{
				StableRuleId:         stableId.String(),
				OrganizationId:       orgId,
				Name:                 rule.Name,
				Description:          rule.Description,
				DisplayOrder:         idx,
				FormulaAstExpression: ruleAst,
				ScoreModifier:        rule.ScoreModifier,
				RuleGroup:            rule.RuleGroup,
			}
		}

		iteration, err := uc.iterationRepository.CreateScenarioIterationAndRules(ctx, tx,
			orgId, models.CreateScenarioIterationInput{
				ScenarioId: scenarioId.String(),
				Body: models.CreateScenarioIterationBody{
					TriggerConditionAstExpression: triggerCondition,
					ScoreReviewThreshold:          scenario.Iteration.ScoreReviewThreshold,
					ScoreBlockAndReviewThreshold:  scenario.Iteration.ScoreBlockAndReviewThreshold,
					ScoreDeclineThreshold:         scenario.Iteration.ScoreDeclineThreshold,
					Schedule:                      scenario.Iteration.Schedule,
					Rules:                         rules,
				},
			})
		if err != nil {
			return err
		}

		for _, sc := range scenario.Iteration.ScreeningConfigs {
			var (
				triggerRule, counterpartyIdExpr *ast.Node
				queries                         map[string]ast.Node
				forcedOutcome                   *models.Outcome
			)

			if sc.TriggerRule != nil {
				tr, err := dto.AdaptASTNode(*sc.TriggerRule)
				if err != nil {
					return err
				}

				triggerRule = &tr

				if err := uc.adaptAstNodeIds(ctx, ids, &tr); err != nil {
					return err
				}
			}
			if sc.CounterpartyIdExpression != nil {
				c, err := dto.AdaptASTNode(*sc.CounterpartyIdExpression)
				if err != nil {
					return err
				}

				counterpartyIdExpr = &c

				if err := uc.adaptAstNodeIds(ctx, ids, &c); err != nil {
					return err
				}
			}

			var err error

			queries = pure_utils.MapKeyValue(sc.Query, func(k string, v dto.NodeDto) (string, ast.Node) {
				q, ierr := dto.AdaptASTNode(v)
				if ierr != nil {
					err = ierr
				}

				return k, q
			})

			if err != nil {
				return err
			}

			if sc.ForcedOutcome != nil {
				forcedOutcome = utils.Ptr(models.OutcomeFrom(*sc.ForcedOutcome))
			}

			_, err = uc.screeningRepository.CreateScreeningConfig(ctx, tx, iteration.Id, models.UpdateScreeningConfigInput{
				Name:                     sc.Name,
				Description:              sc.Description,
				RuleGroup:                sc.RuleGroup,
				Datasets:                 sc.Datasets,
				Threshold:                sc.Threshold,
				TriggerRule:              triggerRule,
				EntityType:               sc.EntityType,
				Query:                    queries,
				CounterpartyIdExpression: counterpartyIdExpr,
				ForcedOutcome:            forcedOutcome,
				Preprocessing:            sc.Preprocessing,
				ConfigVersion:            "v2",
			})
			if err != nil {
				return err
			}
		}

		if err = uc.iterationRepository.UpdateScenarioIterationVersion(ctx, tx, iteration.Id, 1); err != nil {
			return err
		}
		indexes, pending, err := uc.indexEditor.GetIndexesToCreate(ctx, orgId, iteration.Id)
		if err != nil {
			return err
		}

		if len(indexes) > 0 || pending > 0 {
			if err := uc.indexEditor.CreateIndexesBlocking(ctx, orgId, indexes); err != nil {
				return err
			}
		}

		_, err = uc.publicationUsecase.ExecuteScenarioPublicationAction(ctx, orgId, models.PublishScenarioIterationInput{
			ScenarioIterationId: iteration.Id,
			PublicationAction:   models.Publish,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc *OrgImportUsecase) createInboxes(ctx context.Context, tx repositories.Transaction,
	orgId uuid.UUID, ids map[string]string, inboxes []dto.InboxDto,
) error {
	for _, inbox := range inboxes {
		inboxId, _ := uuid.NewV7()
		ids[inbox.Id.String()] = inboxId.String()

		err := uc.inboxRepository.CreateInbox(ctx, tx, models.CreateInboxInput{
			OrganizationId: orgId,
			Name:           inbox.Name,
		}, inboxId)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uc OrgImportUsecase) createWorkflows(
	ctx context.Context,
	tx repositories.Transaction,
	ids map[string]string,
	workflows []dto.ImportWorkflow,
) error {
	for _, workflow := range workflows {
		rule, err := uc.workflowRepository.InsertWorkflowRule(ctx, tx, models.WorkflowRule{
			ScenarioId: uuid.MustParse(ids[workflow.ScenarioId.String()]), Name: workflow.Name,
		})
		if err != nil {
			return err
		}

		ids[workflow.Id.String()] = rule.Id.String()

		for _, cond := range workflow.Conditions {
			params := cond.Params

			switch cond.Function {
			case models.WorkflowConditionRuleHit:
				var p dto.WorkflowConditionRuleHitParams

				if err := json.Unmarshal(cond.Params, &p); err != nil {
					return err
				}

				p.RuleId = pure_utils.Map(p.RuleId, func(id uuid.UUID) uuid.UUID {
					return uuid.MustParse(ids[id.String()])
				})

				params, err = json.Marshal(p)
				if err != nil {
					return err
				}

			case models.WorkflowPayloadEvaluates:
				var p dto.WorkflowConditionEvaluatesParams

				if err := json.Unmarshal(cond.Params, &p); err != nil {
					return err
				}

				if err := uc.adaptAstNodeDtoIds(ctx, ids, &p.Expression); err != nil {
					return err
				}

				params, err = json.Marshal(p)
				if err != nil {
					return err
				}
			}

			if _, err := uc.workflowRepository.InsertWorkflowCondition(ctx, tx, models.WorkflowCondition{
				RuleId: rule.Id, Function: cond.Function, Params: params,
			}); err != nil {
				return err
			}
		}

		for _, action := range workflow.Actions {
			actionType := models.WorkflowTypeFromString(action.Action)
			params := action.Params

			switch actionType {
			case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
				spec, err := models.ParseWorkflowAction[dto.WorkflowActionCaseParams](models.WorkflowAction{
					Action: actionType, Params: action.Params,
				})
				if err != nil {
					return err
				}

				spec.Params.InboxId = uuid.MustParse(ids[spec.Params.InboxId.String()])
				spec.Params.TagsToAdd = pure_utils.Map(spec.Params.TagsToAdd, func(id uuid.UUID) uuid.UUID {
					return uuid.MustParse(ids[id.String()])
				})

				params, err = json.Marshal(spec.Params)
				if err != nil {
					return err
				}
			}

			if _, err := uc.workflowRepository.InsertWorkflowAction(ctx, tx, models.WorkflowAction{
				RuleId: rule.Id, Action: models.WorkflowType(action.Action), Params: params,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (uc *OrgImportUsecase) adaptAstNodeIds(ctx context.Context, ids map[string]string, node *ast.Node) error {
	if node.Function == ast.FUNC_CUSTOM_LIST_ACCESS {
		args, ok := node.NamedChildren["customListId"].Constant.(string)
		if !ok {
			return errors.New("FUNC_CUSTOM_LIST_ACCESS requires a `customListId` named parameter")
		}

		node.NamedChildren["customListId"] = ast.Node{Constant: ids[args]}
	}

	for _, child := range node.Children {
		if err := uc.adaptAstNodeIds(ctx, ids, &child); err != nil {
			return err
		}
	}

	return nil
}

func (uc OrgImportUsecase) adaptAstNodeDtoIds(ctx context.Context, ids map[string]string, node *dto.NodeDto) error {
	astNode, err := dto.AdaptASTNode(*node)
	if err != nil {
		return err
	}

	if err := uc.adaptAstNodeIds(ctx, ids, &astNode); err != nil {
		return err
	}

	*node, err = dto.AdaptNodeDto(astNode)
	if err != nil {
		return err
	}

	return nil
}
