package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestValidateWorkflowCondition(t *testing.T) {
	tts := []struct {
		valid  bool
		fn     models.WorkflowConditionType
		params string
	}{
		{false, "anything", ""},
		{true, models.WorkflowConditionAlways, ""},
		{false, models.WorkflowConditionAlways, `"anything"`},
		{true, models.WorkflowConditionNever, ""},
		{false, models.WorkflowConditionNever, `"anything"`},
		{false, models.WorkflowConditionOutcomeIn, ""},
		{false, models.WorkflowConditionOutcomeIn, `[]`},
		{false, models.WorkflowConditionOutcomeIn, `["nop", "decline"]`},
		{true, models.WorkflowConditionOutcomeIn, `["review", "block_and_review"]`},
		{false, models.WorkflowConditionRuleHit, `"anything`},
		{false, models.WorkflowConditionRuleHit, `{}`},
		{true, models.WorkflowConditionRuleHit, `{"rule_id": []}`},
		{false, models.WorkflowConditionRuleHit, `{"rule_id": "anything"}`},
		{false, models.WorkflowConditionRuleHit, `{"rule_id": "337331bd-3a0c-44cf-ab5b-3f62aa7bcd44"}`},
		{true, models.WorkflowConditionRuleHit, `{"rule_id": ["337331bd-3a0c-44cf-ab5b-3f62aa7bcd44"]}`},
		{false, models.WorkflowPayloadEvaluates, ``},
		{false, models.WorkflowPayloadEvaluates, `"anything"`},
		{false, models.WorkflowPayloadEvaluates, `{"expression":{{"no": "ast"}}`},
		{false, models.WorkflowPayloadEvaluates, `{"expression":{{"constant": "string"}}`},
		{true, models.WorkflowPayloadEvaluates, `{"expression":{"constant": true}}`},
	}

	scenario := models.Scenario{TriggerObjectType: "transactions"}
	exec, astValidator := makeScenarioEvaluator(t, scenario)
	uc := WorkflowUsecase{
		executorFactory: exec,
		validateScenarioAst: &scenarios.ValidateScenarioAstImpl{
			AstValidator: astValidator,
		},
	}

	for _, tt := range tts {
		var params []byte

		if tt.params != "" {
			params = []byte(tt.params)
		}

		err := uc.ValidateWorkflowCondition(t.Context(), scenario,
			models.WorkflowCondition{Function: tt.fn, Params: params})

		switch tt.valid {
		case true:
			assert.NoError(t, err)
		case false:
			assert.Error(t, err)
		}
	}
}

func TestValidateWorkflowAction(t *testing.T) {
	tts := []struct {
		valid  bool
		fn     models.WorkflowType
		params string
	}{
		{false, "anything", ""},
		{false, models.WorkflowCreateCase, ""},
		{false, models.WorkflowCreateCase, "anything"},
		{false, models.WorkflowCreateCase, `{}`},
		{true, models.WorkflowCreateCase, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82"}`},
		{true, models.WorkflowCreateCase, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","any_inbox":true}`},
		{false, models.WorkflowCreateCase, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":""}`},
		{false, models.WorkflowCreateCase, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":{"constant":12}}`},
		{true, models.WorkflowCreateCase, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":{"constant":"title"}}`},
		{true, models.WorkflowAddToCaseIfPossible, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82"}`},
		{true, models.WorkflowAddToCaseIfPossible, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","any_inbox":true}`},
		{false, models.WorkflowAddToCaseIfPossible, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":""}`},
		{false, models.WorkflowAddToCaseIfPossible, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":{"constant":12}}`},
		{true, models.WorkflowAddToCaseIfPossible, `{"inbox_id":"bc95e413-9096-4146-b68c-39cf2b2b9b82","title_template":{"constant":"title"}}`},
	}

	scenario := models.Scenario{TriggerObjectType: "transactions"}
	exec, astValidator := makeScenarioEvaluator(t, scenario)
	uc := WorkflowUsecase{
		executorFactory: exec,
		validateScenarioAst: &scenarios.ValidateScenarioAstImpl{
			AstValidator: astValidator,
		},
	}

	for _, tt := range tts {
		var params []byte

		if tt.params != "" {
			params = []byte(tt.params)
		}

		err := uc.ValidateWorkflowAction(t.Context(), scenario,
			models.WorkflowAction{Action: tt.fn, Params: params})

		switch tt.valid {
		case true:
			assert.NoError(t, err)
		case false:
			assert.Error(t, err)
		}
	}
}

func makeScenarioEvaluator(t *testing.T, scenario models.Scenario) (executor_factory.ExecutorFactory, scenarios.AstValidator) {
	ctx := t.Context()

	exec := new(mocks.Executor)
	executorFactory := new(mocks.ExecutorFactory)
	executorFactory.On("NewExecutor").Return(exec)

	dataModel := new(mocks.DataModelRepository)
	dataModel.On("GetDataModel", ctx, exec, scenario.OrganizationId, false, mock.Anything).
		Return(models.DataModel{
			Version: "1",
			Tables: map[string]models.Table{
				"transactions": {
					Name: "transactions",
					Fields: map[string]models.Field{
						"id": {DataType: models.Int},
					},
				},
			},
		}, nil)

	validator := scenarios.AstValidatorImpl{
		DataModelRepository: dataModel,
		AstEvaluationEnvironmentFactory: func(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
			return ast_eval.NewAstEvaluationEnvironment().WithoutOptimizations()
		},
		ExecutorFactory: executorFactory,
	}

	return executorFactory, &validator
}
