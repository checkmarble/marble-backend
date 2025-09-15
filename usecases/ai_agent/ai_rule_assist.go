package ai_agent

import (
	"context"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/marble-backend/dto/agent_dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	RULE_DESCRIPTION_PROMPT_PATH = "prompts/rule/rule_description.md"
)

type aiRuleDescriptionOutput struct {
	Description string  `json:"description" jsonschema_description:"The description of the rule"`
	Example     string  `json:"example" jsonschema_description:"The example of the rule with explanation"`
	Advice      *string `json:"advice" jsonschema_description:"The advice to the user to improve the rule, could be empty"`
}

func (uc *AiAgentUsecase) AiRuleDescription(
	ctx context.Context,
	orgId string,
	ruleId string,
) (models.AiRuleDescription, error) {
	exec := uc.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	// Get the scenario iteration
	// Permissions are checked in the rule usecase
	rule, err := uc.ruleUsecase.GetRule(ctx, ruleId)
	if err != nil {
		return models.AiRuleDescription{}, err
	}
	ruleDto, err := agent_dto.AdaptRuleDto(rule)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	// Get custom list (at least list of custom list with their names and ID)
	customLists, err := uc.customListRepository.AllCustomLists(ctx, exec, orgId)
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
	adapter, err := uc.GetClient(ctx)
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	// Execute the LLM prompt and return the result
	model, ruleDescription, err := uc.preparePromptWithModel(RULE_DESCRIPTION_PROMPT_PATH, map[string]any{
		"data_model":  dataModelDto,
		"custom_list": customListsDto,
		"rule":        ruleDto,
	})
	if err != nil {
		return models.AiRuleDescription{}, err
	}

	logger.DebugContext(ctx, "Rule description", "prompt", ruleDescription)

	aiStudioRequest, err := llmberjack.NewRequest[aiRuleDescriptionOutput]().
		WithModel(model).
		WithText(llmberjack.RoleUser, ruleDescription).
		Do(ctx, adapter)
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
		Example:     ruleDescriptionResponse.Example,
		Advice:      ruleDescriptionResponse.Advice,
	}, nil
}
