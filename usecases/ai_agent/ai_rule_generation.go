package ai_agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

const (
	RULE_GENERATION_PROMPT_PATH = "prompts/rule/rule_generation.md"
)

// GenerateRule generates a rule AST from a natural language instruction
// Does NOT persist to database - returns AST + validation for frontend to decide
func (uc *AiAgentUsecase) GenerateRule(
	ctx context.Context,
	orgId uuid.UUID,
	ruleId string,
	instruction string,
) (dto.GenerateRuleResponse, error) {
	logger := utils.LoggerFromContext(ctx)
	exec := uc.executorFactory.NewExecutor()

	// Step 1: Fetch rule (permission check in rule usecase)
	rule, err := uc.ruleUsecase.GetRule(ctx, ruleId)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 2: Fetch scenario and iteration
	scenarioAndIteration, err := uc.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, rule.ScenarioIterationId)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 3: Fetch data model
	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeEnums:             true,
		IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 4: Fetch custom lists
	customLists, err := uc.customListUsecase.GetCustomLists(ctx, orgId)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 5: Extract available identifiers
	databaseAccessors, err := getLinkedDatabaseIdentifiers(scenarioAndIteration.Scenario, dataModel)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	payloadAccessors, err := getPayloadIdentifiers(scenarioAndIteration.Scenario, dataModel)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 6: Build prompt and call LLM
	client, err := uc.GetClient(ctx)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	dataModelDto := agent_dto.AdaptDataModelDto(dataModel)
	customListsDto := pure_utils.Map(customLists, agent_dto.AdaptCustomListDto)

	databaseNodes, err := pure_utils.MapErr(databaseAccessors, dto.AdaptNodeDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	payloadNodes, err := pure_utils.MapErr(payloadAccessors, dto.AdaptNodeDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	// Step 7: Prepare prompt with model
	model, ruleGenerationPrompt, err := uc.preparePromptWithModel(RULE_GENERATION_PROMPT_PATH, map[string]any{
		"data_model":         dataModelDto,
		"custom_list":        customListsDto,
		"instruction":        instruction,
		"trigger_type":       scenarioAndIteration.Scenario.TriggerObjectType,
		"database_accessors": databaseNodes,
		"payload_accessors":  payloadNodes,
	})
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}

	logger.DebugContext(ctx, "Generating rule", "model", model, "prompt_length", len(ruleGenerationPrompt))

	// Step 8: Create JSON schema for NodeDto (recursive)
	nodeSchema := buildNodeDtoSchema()
	jsschema, err := json.Marshal(nodeSchema)
	if err != nil {
		return dto.GenerateRuleResponse{}, err
	}
	_ = jsschema // used for logging if needed

	// Step 9: Call LLM with structured output
	req, err := llmberjack.NewRequest[dto.NodeDto]().
		WithModel(model).
		WithSchemaDescription("NodeDto", "The AST node of the rule").
		OverrideResponseSchema(nodeSchema).
		WithText(llmberjack.RoleUser, ruleGenerationPrompt).
		WithThinking(true).
		Do(ctx, client)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to generate rule from LLM: %w", err)
	}

	// Step 10: Extract generated NodeDto
	ruleAstDto, err := req.Get(0)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	logger.DebugContext(ctx, "Generated rule AST", "ast_dto", ruleAstDto)

	// Step 11: Convert to AST
	ruleAst, err := dto.AdaptASTNode(ruleAstDto)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to adapt AST node: %w", err)
	}

	// Step 12: Validate generated AST
	astValidation, err := uc.scenarioUsecase.ValidateScenarioAst(ctx,
		scenarioAndIteration.Scenario.Id, &ruleAst)
	if err != nil {
		return dto.GenerateRuleResponse{}, fmt.Errorf("failed to validate generated AST: %w", err)
	}

	// Step 13: Build response with validation details
	var validationErrors []string

	// Convert ScenarioValidationError to strings
	for _, err := range astValidation.Errors {
		validationErrors = append(validationErrors, err.Error.Error())
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

// buildNodeDtoSchema creates a JSON schema for recursive NodeDto
func buildNodeDtoSchema() jsonschema.Schema {
	properties := jsonschema.NewProperties()
	properties.Set("name", &jsonschema.Schema{
		Type:        "string",
		Description: "The function name or constant name",
	})
	properties.Set("constant", &jsonschema.Schema{
		Type: "string",
	})
	properties.Set("children", &jsonschema.Schema{
		Type:        "array",
		Description: "Ordered children nodes",
		Items: &jsonschema.Schema{
			Ref: "#/definitions/NodeDto",
		},
	})
	properties.Set("named_children", &jsonschema.Schema{
		Type:        "object",
		Description: "Named children nodes (for specific node types)",
		PatternProperties: map[string]*jsonschema.Schema{
			"^.*$": {
				Ref: "#/definitions/NodeDto",
			},
		},
		AdditionalProperties: jsonschema.FalseSchema,
	})

	schema := jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Definitions: jsonschema.Definitions{
			"NodeDto": {
				Type:                 "object",
				Properties:           properties,
				AdditionalProperties: jsonschema.FalseSchema,
				Required:             []string{"name", "constant", "children"},
			},
		},
		AdditionalProperties: jsonschema.FalseSchema,
		Required:             []string{"name", "constant", "children"},
	}

	return schema
}
