package evaluate

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories/mocks"
	"marble/marble-backend/usecases/security"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCustomListValuesWrongArg(t *testing.T) {
	customListEval := NewCustomListValuesAccess(nil, nil)
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
	customListEval := NewCustomListValuesAccess(mockCustomListRepo, mockEnforceSecurity)

	testCustomListValues := []models.CustomListValue{{Value: "test"}, {Value: "test2"}}

	mockCustomListRepo.EXPECT().GetCustomListById(nil, TestListId).Return(TestList, nil)
	mockCustomListRepo.EXPECT().GetCustomListValues(nil, models.GetCustomListValuesInput{
		Id: TestListId,
	}).Return(testCustomListValues, nil)
	mockEnforceSecurity.EXPECT().ReadOrganization(TestListOrgId).Return(nil)

	result, errs := customListEval.Evaluate(ast.Arguments{NamedArgs: TestNamedArgs})
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
	customListEval := NewCustomListValuesAccess(mockCustomListRepo, mockEnforceSecurity)

	mockCustomListRepo.EXPECT().GetCustomListById(nil, TestListId).Return(TestList, nil)
	mockEnforceSecurity.EXPECT().ReadOrganization(TestListOrgId).Return(models.ForbiddenError)

	_, errs := customListEval.Evaluate(ast.Arguments{NamedArgs: TestNamedArgs})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], models.ForbiddenError)
	}
}
