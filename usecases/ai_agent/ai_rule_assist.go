package ai_agent

import (
	"context"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const (
	RULE_DESCRIPTION_PROMPT_PATH = "prompts/rule/rule_description.md"
)

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
	logger := utils.LoggerFromContext(ctx)

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
	model, ruleDescription, err := uc.preparePromptWithModel(RULE_DESCRIPTION_PROMPT_PATH, map[string]any{
		"data_model":  dataModelDto,
		"custom_list": customListsDto,
		"rule":        ruleAST,
	})
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	logger.DebugContext(ctx, "Rule description", "model", model)
	logger.DebugContext(ctx, "Rule description", "prompt", ruleDescription)

	aiStudioRequest, err := llmberjack.NewRequest[aiRuleDescriptionOutput]().
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

	logger.DebugContext(ctx, "Rule description", "response", ruleDescriptionResponse)

	return models.AiRuleDescription{
		Description: ruleDescriptionResponse.Description,
		IsRuleValid: true,
	}, nil
}
