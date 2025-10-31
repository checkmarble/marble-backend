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
	"github.com/invopop/jsonschema"
)

const (
	RULE_DESCRIPTION_PROMPT_PATH = "prompts/rule/rule_description.md"
	RULE_GENERATION_PROMPT_PATH  = "prompts/rule/rule_generation.md"
)

type aiRuleDescriptionOutput struct {
	Description string `json:"description" jsonschema_description:"The description of the rule"`
}

func (uc *AiAgentUsecase) AiRuleDescription(
	ctx context.Context,
	orgId string,
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

// TODO: not production ready, do not put in prod before a full review of permissions checks and other side effects to consider.
// In particular, probably do not put in prod an endpoint that mutates the rule by id.
func (uc *AiAgentUsecase) GenerateAstRule(
	ctx context.Context,
	orgId string,
	ruleId string,
	instruction string,
) error {
	logger := utils.LoggerFromContext(ctx)
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.ruleUsecase.GetRule(ctx, ruleId)
	if err != nil {
		return err
	}
	scenarioAndIteration, err := uc.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, rule.ScenarioIterationId)
	if err != nil {
		return err
	}

	// Get custom list (at least list of custom list with their names and ID)
	customLists, err := uc.customListUsecase.GetCustomLists(ctx, orgId)
	if err != nil {
		return err
	}
	customListsDto := pure_utils.Map(customLists, agent_dto.AdaptCustomListDto)

	// Get the data model
	dataModel, err := uc.dataModelUsecase.GetDataModel(ctx, orgId, models.DataModelReadOptions{
		IncludeEnums: true, IncludeNavigationOptions: true,
	}, true)
	if err != nil {
		return err
	}
	dataModelDto := agent_dto.AdaptDataModelDto(dataModel)

	client, err := uc.GetClient(ctx)
	if err != nil {
		return err
	}

	model, ruleGenerationPrompt, err := uc.preparePromptWithModel(RULE_GENERATION_PROMPT_PATH, map[string]any{
		"data_model":   dataModelDto,
		"custom_list":  customListsDto,
		"instruction":  instruction,
		"trigger_type": scenarioAndIteration.Scenario.TriggerObjectType,
	})
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Rule generation", "model", model)
	// logger.DebugContext(ctx, "Rule generation", "prompt", ruleGenerationPrompt)

	properties := jsonschema.NewProperties()
	properties.Set("name", &jsonschema.Schema{
		Type: "string",
	})
	properties.Set("constant", &jsonschema.Schema{
		Type: "string",
	})
	properties.Set("children", &jsonschema.Schema{
		Type: "array",
		Items: &jsonschema.Schema{
			Ref: "#/definitions/NodeDto",
		},
	})
	// properties.Set("named_children", &jsonschema.Schema{
	// 	Type: "object",
	// 	PatternProperties: map[string]*jsonschema.Schema{
	// 		"^.*$": {
	// 			Ref: "#/definitions/NodeDto",
	// 		},
	// 	},
	// 	AdditionalProperties: nil,
	// })

	rootProps := jsonschema.NewProperties()
	rootProps.Set("root", &jsonschema.Schema{
		Type: "object",
		Ref:  "#/definitions/NodeDto",
	})
	schema := jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		// Ref:        "#/definitions/NodeDto",
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

	jsschema, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	fmt.Println(string(jsschema))

	// Name          string             `json:"name,omitempty" jsonschema_description:"Name of the AST node" jsonschema:"required"`
	// Constant      any                `json:"constant,omitempty" jsonschema_description:"Constant value of the node, if the node is a constant"`
	// Children      []NodeDto          `json:"children,omitempty" jsonschema_description:"Positional arguments to the current function"`
	// NamedChildren map[string]NodeDto `json:"named_children,omitempty" jsonschema_description:"Name arguments to the current function"`

	// Boom stack overflow
	aiStudioRequest, err := llmberjack.NewRequest[dto.NodeDto]().
		// aiStudioRequest, err := llmberjack.NewRequest[string]().
		WithModel(model).
		WithSchemaDescription("NodeDto", "The AST node of the rule").
		OverrideResponseSchema(schema).
		WithText(llmberjack.RoleUser, ruleGenerationPrompt).
		// WithText(llmberjack.RoleUser, "Create an AST tree using the SUM function with two positional children that scalar of the INT nodes 3 and 5").
		WithThinking(true).
		Do(ctx, client)
	if err != nil {
		return err
	}

	// that's with the proper type...
	ruleAstDto, err := aiStudioRequest.Get(0)
	if err != nil {
		return err
	}

	// dtoString, err := aiStudioRequest.Get(0)
	// if err != nil {
	// 	return err
	// }
	// var ruleAstDto dto.NodeDto
	// err = json.Unmarshal([]byte(dtoString), &ruleAstDto)
	// if err != nil {
	// 	return err
	// }

	ruleAst, err := dto.AdaptASTNode(ruleAstDto)
	if err != nil {
		return err
	}
	astValidation, err := uc.scenarioUsecase.ValidateScenarioAst(ctx,
		scenarioAndIteration.Scenario.Id, &ruleAst)
	if err != nil {
		return err
	}

	logger.DebugContext(ctx, "Rule generation", "response", ruleAstDto)
	logger.DebugContext(ctx, "Rule generation", "ast validation", astValidation.Evaluation)
	logger.DebugContext(ctx, "Rule generation", "ast validation errors",
		astValidation.Evaluation.FlattenErrors())

	err = uc.repository.UpdateRule(ctx, exec, models.UpdateRuleInput{
		Id:                   ruleId,
		FormulaAstExpression: &ruleAst,
	})
	if err != nil {
		return err
	}

	return nil
}

// AiASTDescription generates a description for a given AST node
// Requires permissions to read the data model and custom lists
// Require permission to read scenario (needed for ast validation)
func (uc *AiAgentUsecase) AiASTDescription(
	ctx context.Context,
	orgId string,
	scenarioId string,
	ruleAST *ast.Node,
) (models.AiRuleDescription, error) {
	// logger := utils.LoggerFromContext(ctx)

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

	// logger.DebugContext(ctx, "Rule description", "model", model)
	// logger.DebugContext(ctx, "Rule description", "prompt", ruleDescription)

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

	// logger.DebugContext(ctx, "Rule description", "response", ruleDescriptionResponse)

	return models.AiRuleDescription{
		Description: ruleDescriptionResponse.Description,
		IsRuleValid: true,
	}, nil
}
