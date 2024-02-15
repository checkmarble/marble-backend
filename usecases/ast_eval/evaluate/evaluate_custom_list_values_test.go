package evaluate_test

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"

	"github.com/stretchr/testify/assert"
)

// For Custom List Evaluator
const (
	testListId    string = "1"
	testListOrgId string = "2"
)

var testList = models.CustomList{
	Id:             testListId,
	OrganizationId: testListOrgId,
}

var testCustomListNamedArgs = map[string]any{
	"customListId": testListId,
}

func TestCustomListValuesWrongArg(t *testing.T) {
	execFactory := new(mocks.ExecutorFactory)
	execFactory.On("NewExecutor").Return(new(mocks.Executor))
	customListEval := evaluate.NewCustomListValuesAccess(nil, nil, execFactory)
	_, errs := customListEval.Evaluate(context.TODO(), ast.Arguments{Args: []any{true}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrMissingNamedArgument)
	}
}

func TestCustomListValues(t *testing.T) {
	clr := new(mocks.CustomListRepository)
	er := new(mocks.EnforceSecurity)
	execFactory := new(mocks.ExecutorFactory)
	exec := new(mocks.Executor)

	customListEval := evaluate.NewCustomListValuesAccess(clr, er, execFactory)

	testCustomListValues := []models.CustomListValue{{Value: "test"}, {Value: "test2"}}

	execFactory.On("NewExecutor").Return(exec)
	clr.On("GetCustomListById", exec, testListId).Return(testList, nil)
	clr.On("GetCustomListValues", exec, models.GetCustomListValuesInput{Id: testListId}).Return(testCustomListValues, nil)

	er.On("ReadOrganization", testListOrgId).Return(nil)
	result, errs := customListEval.Evaluate(context.TODO(), ast.Arguments{
		NamedArgs: testCustomListNamedArgs,
	})
	assert.Len(t, errs, 0)
	if assert.Len(t, result, 2) {
		assert.Equal(t, result.([]string)[0], testCustomListValues[0].Value)
		assert.Equal(t, result.([]string)[1], testCustomListValues[1].Value)
	}

	clr.AssertExpectations(t)
	er.AssertExpectations(t)
}

func TestCustomListValuesNoAccess(t *testing.T) {
	clr := new(mocks.CustomListRepository)
	er := new(mocks.EnforceSecurity)
	execFactory := new(mocks.ExecutorFactory)
	exec := new(mocks.Executor)

	customListEval := evaluate.NewCustomListValuesAccess(clr, er, execFactory)

	execFactory.On("NewExecutor").Return(exec)
	clr.On("GetCustomListById", exec, testListId).Return(testList, nil)
	er.On("ReadOrganization", testListOrgId).Return(models.ForbiddenError)

	_, errs := customListEval.Evaluate(context.TODO(), ast.Arguments{NamedArgs: testCustomListNamedArgs})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], models.ForbiddenError)
	}

	clr.AssertExpectations(t)
	er.AssertExpectations(t)
}
