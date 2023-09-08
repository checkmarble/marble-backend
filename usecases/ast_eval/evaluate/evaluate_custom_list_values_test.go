package evaluate_test

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories/mocks"
	"marble/marble-backend/usecases/ast_eval/evaluate"
	"marble/marble-backend/usecases/security"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCustomListValuesWrongArg(t *testing.T) {
	customListEval := evaluate.NewCustomListValuesAccess(nil, nil)
	_, errs := customListEval.Evaluate(ast.Arguments{Args: []any{true}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrMissingNamedArgument)
	}
}

func TestCustomListValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockCustomListRepo := mocks.NewMockCustomListRepository(ctrl)
	mockEnforceSecurity := security.NewMockEnforceSecurity(ctrl)
	customListEval := evaluate.NewCustomListValuesAccess(mockCustomListRepo, mockEnforceSecurity)

	testCustomListValues := []models.CustomListValue{{Value: "test"}, {Value: "test2"}}

	mockCustomListRepo.EXPECT().GetCustomListById(nil, testListId).Return(testList, nil)
	mockCustomListRepo.EXPECT().GetCustomListValues(nil, models.GetCustomListValuesInput{
		Id: testListId,
	}).Return(testCustomListValues, nil)
	mockEnforceSecurity.EXPECT().ReadOrganization(testListOrgId).Return(nil)

	result, errs := customListEval.Evaluate(ast.Arguments{NamedArgs: testCustomListNamedArgs})
	assert.Len(t, errs, 0)
	if assert.Len(t, result, 2) {
		assert.Equal(t, result.([]string)[0], testCustomListValues[0].Value)
		assert.Equal(t, result.([]string)[1], testCustomListValues[1].Value)
	}
}

func TestCustomListValuesNoAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockCustomListRepo := mocks.NewMockCustomListRepository(ctrl)
	mockEnforceSecurity := security.NewMockEnforceSecurity(ctrl)
	customListEval := evaluate.NewCustomListValuesAccess(mockCustomListRepo, mockEnforceSecurity)

	mockCustomListRepo.EXPECT().GetCustomListById(nil, testListId).Return(testList, nil)
	mockEnforceSecurity.EXPECT().ReadOrganization(testListOrgId).Return(models.ForbiddenError)

	_, errs := customListEval.Evaluate(ast.Arguments{NamedArgs: testCustomListNamedArgs})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], models.ForbiddenError)
	}
}
