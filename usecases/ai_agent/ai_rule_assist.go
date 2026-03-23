package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const (
	RULE_DESCRIPTION_PROMPT_PATH      = "prompts/rule/rule_description.md"
	RULE_GENERATION_PROMPT_STEP1_PATH = "prompts/rule/rule_generation_step1.md"
	RULE_GENERATION_PROMPT_STEP2_PATH = "prompts/rule/rule_generation_step2.md"
)

// GenerateRule generates a rule AST from a natural language instruction
// Returns AST + validation without persisting to database (frontend controls save)
func (uc *AiAgentUsecase) GenerateRule(
	ctx context.Context,
	orgId uuid.UUID,
	scenarioId string,
	instruction string,
) (dto.GenerateRuleResponse, error) {
	logger := utils.LoggerFromContext(ctx)

	featureAccess, err := uc.featureAccessReader.GetOrganizationFeatureAccess(ctx, orgId, nil)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	if !featureAccess.AiRuleBuilding.IsAllowed() {
		return dto.GenerateRuleResponse{}, models.ForbiddenError
	}

	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	if err := uc.enforceSecurityScenario.ReadScenario(scenario); err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	customLists, err := uc.customListUsecase.GetCustomLists(ctx, orgId)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	customListsDto := pure_utils.Map(customLists, agent_dto.AdaptCustomListDto)

	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	dataModelDto := agent_dto.AdaptDataModelDto(dataModel)

	client, err := uc.GetClient(ctx)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	databaseAccessors, err := models.GetLinkedDatabaseIdentifiers(scenario, dataModel)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	databaseNodes, err := pure_utils.MapErr(databaseAccessors, dto.AdaptNodeDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	payloadAccessors, err := models.GetPayloadIdentifiers(scenario, dataModel)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	payloadNodes, err := pure_utils.MapErr(payloadAccessors, dto.AdaptNodeDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	provider, model, ruleGenerationPrompt, err := uc.preparePromptWithModel(
		RULE_GENERATION_PROMPT_STEP1_PATH, map[string]any{
			"data_model":         dataModelDto,
			"custom_list":        customListsDto,
			"instruction":        instruction,
			"trigger_type":       scenario.TriggerObjectType,
			"database_accessors": databaseNodes,
			"payload_accessors":  payloadNodes,
		})
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	resp, err := llmberjack.NewRequest[string]().
		WithProvider(provider).
		WithModel(model).
		WithText(llmberjack.RoleUser, ruleGenerationPrompt).
		WithThinking(true).
		Do(ctx, client)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf(
			"failed to generate rule from LLM: %w", err)
	}

	ruleAsString, err := resp.Get(0)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to get LLM response: %w", err)
	}

	logger.DebugContext(ctx, fmt.Sprintf("LLM response as string:\n%s\n", ruleAsString))

	provider, model, ruleGenerationPrompt, err = uc.preparePromptWithModel(
		RULE_GENERATION_PROMPT_STEP2_PATH, map[string]any{
			"data_model":         dataModelDto,
			"custom_list":        customListsDto,
			"instruction":        instruction,
			"trigger_type":       scenario.TriggerObjectType,
			"database_accessors": databaseNodes,
			"payload_accessors":  payloadNodes,
			"rule_plan":          ruleAsString,
		})
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	resp, err = llmberjack.NewRequest[string]().
		WithProvider(provider).
		WithModel(model).
		WithSchemaDescription("NodeDto", "The AST node of the rule").
		WithText(llmberjack.RoleUser, ruleGenerationPrompt).
		Do(ctx, client)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf(
			"failed to generate rule from LLM: %w", err)
	}

	ruleAsStringStep2, err := resp.Get(0)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to get LLM response: %w", err)
	}

	logger.DebugContext(ctx, fmt.Sprintf("LLM response step 2 as json string:\n%s\n", ruleAsStringStep2))

	var ruleAstDto dto.NodeDto
	err = json.Unmarshal([]byte(ruleAsStringStep2), &ruleAstDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf(
			"failed to parse LLM response as JSON: %w", err)
	}

	ruleAst, err := dto.AdaptASTNode(ruleAstDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to adapt AST node: %w", err)
	}

	astValidation, err := uc.scenarioUsecase.ValidateScenarioAst(ctx,
		scenario.Id, &ruleAst)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf(
			"failed to validate generated AST: %w", err)
	}

	// Build response with validation details
	var validationErrors []string

	// Convert ScenarioValidationError to strings
	for _, validErr := range astValidation.Errors {
		validationErrors = append(validationErrors, validErr.Error.Error())
	}

	// Convert evaluation errors to strings
	evaluationErrors := astValidation.Evaluation.FlattenErrors()
	for _, err := range evaluationErrors {
		validationErrors = append(validationErrors, err.Error())
	}

	isValid := len(validationErrors) == 0

	logger.DebugContext(ctx, "AST validation result",
		"is_valid", isValid,
		"errors_count", len(validationErrors),
	)

	response := dto.GenerateRuleResponse{
		RuleAST: &ruleAstDto,
		Validation: dto.ASTValidationDetail{
			IsValid:  isValid,
			Errors:   validationErrors,
			Warnings: []string{},
		},
	}

	return response, nil
}

type aiRuleDescriptionOutput struct {
	Description string `json:"description" jsonschema_description:"The description of the rule"`
}

func (uc *AiAgentUsecase) AiRuleDescription(
	ctx context.Context,
	orgId uuid.UUID,
	ruleId string,
) (models.AiRuleDescription, error) {
	// Get the scenario iteration
	// Permissions are checked in the rule usecase
	rule, err := uc.ruleUsecase.GetRule(ctx, ruleId)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	scenarioIteration, err := uc.repository.GetScenarioIteration(
		ctx,
		uc.executorFactory.NewExecutor(),
		rule.ScenarioIterationId,
		true,
	)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	return uc.AiASTDescription(ctx, orgId, scenarioIteration.ScenarioId, rule.FormulaAstExpression)
}

// AiASTDescription generates a description for a given AST node
// Requires permissions to read the data model and custom lists
// Require permission to read scenario (needed for ast validation)
func (uc *AiAgentUsecase) AiASTDescription(
	ctx context.Context,
	orgId uuid.UUID,
	scenarioId string,
	ruleAST *ast.Node,
) (models.AiRuleDescription, error) {
	// Check if the rule is valid before calling LLM
	astValidation, err := uc.scenarioUsecase.ValidateScenarioAst(ctx, scenarioId, ruleAST)
	if err != nil {
		return models.AiRuleDescription{}, err
	}
	// Check if the evaluation has errors
	flattenErrors := astValidation.Evaluation.FlattenErrors()
	if len(astValidation.Errors) > 0 || len(flattenErrors) > 0 {
		return models.AiRuleDescription{
			IsRuleValid: false,
		}, nil
	}

	// Get custom list (at least list of custom list with their names and ID)
	customLists, err := uc.customListUsecase.GetCustomLists(ctx, orgId)
	if err != nil {
		return models.AiRuleDescription{}, err
	}
	customListsDto := pure_utils.Map(customLists, agent_dto.AdaptCustomListDto)

	// Get the data model
	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return models.AiRuleDescription{}, err
	}
	dataModelDto := agent_dto.AdaptDataModelDto(dataModel)

	// Get the LLM client and prompt
	client, err := uc.GetClient(ctx)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	// Execute the LLM prompt and return the result
	provider, model, ruleDescription, err := uc.preparePromptWithModel(
		RULE_DESCRIPTION_PROMPT_PATH, map[string]any{
			"data_model":  dataModelDto,
			"custom_list": customListsDto,
			"rule":        ruleAST,
		})
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	aiStudioRequest, err := llmberjack.NewRequest[aiRuleDescriptionOutput]().
		WithProvider(provider).
		WithModel(model).
		WithText(llmberjack.RoleUser, ruleDescription).
		WithThinking(false).
		Do(ctx, client)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	ruleDescriptionResponse, err := aiStudioRequest.Get(0)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	return models.AiRuleDescription{
		Description: ruleDescriptionResponse.Description,
		IsRuleValid: true,
	}, nil
}
