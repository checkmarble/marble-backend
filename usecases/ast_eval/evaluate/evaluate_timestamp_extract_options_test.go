package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTimestampExtractOptionsEvaluator_Evaluate(t *testing.T) {
	tests := []struct {
		name             string
		organizationTime *string
		expectedTimezone string
	}{
		{
			name:             "organization timezone",
			organizationTime: utils.Ptr("Europe/Paris"),
			expectedTimezone: "Europe/Paris",
		},
		{
			name:             "UTC fallback",
			expectedTimezone: "UTC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			orgID := uuid.MustParse("0193721e-88d9-7f67-9221-f7fbeb1a1e9e")
			orgRepo := new(mocks.OrganizationRepository)
			execFactory := new(mocks.ExecutorFactory)
			exec := new(mocks.Transaction)
			execFactory.On("NewExecutor").Once().Return(exec)
			orgRepo.On("GetOrganizationById", ctx, exec, orgID).Once().Return(models.Organization{
				Id:                      orgID,
				DefaultScenarioTimezone: tt.organizationTime,
			}, nil)
			evaluator := NewTimestampExtractOptionsEvaluator(execFactory, orgRepo, orgID)

			result, errs := evaluator.Evaluate(ctx, ast.Arguments{NamedArgs: map[string]any{
				"part":   "hour",
				"ranges": []any{[]any{8, 12}, []any{14, 18}},
			}})

			assert.Empty(t, errs)
			assert.Equal(t, ast.TimestampExtractOptions{
				Part:     "hour",
				Ranges:   [][2]int{{8, 12}, {14, 18}},
				Timezone: tt.expectedTimezone,
			}, result)
			execFactory.AssertExpectations(t)
			orgRepo.AssertExpectations(t)
		})
	}
}

func TestTimestampExtractOptionsEvaluator_RejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name          string
		namedArgs     map[string]any
		expectedError string
	}{
		{
			name: "invalid part",
			namedArgs: map[string]any{
				"part":   "minute",
				"ranges": []any{[]any{1, 2}},
			},
			expectedError: "part",
		},
		{
			name: "reversed range",
			namedArgs: map[string]any{
				"part":   "hour",
				"ranges": []any{[]any{18, 8}},
			},
			expectedError: "ranges",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, errs := (TimestampExtractOptionsEvaluator{}).Evaluate(
				context.Background(),
				ast.Arguments{NamedArgs: tt.namedArgs},
			)

			assert.Nil(t, result)
			if assert.Len(t, errs, 1) {
				assert.ErrorContains(t, errs[0], tt.expectedError)
			}
		})
	}
}
