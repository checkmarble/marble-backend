package evaluate

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/stretchr/testify/assert"
)

func TestTimestampExtract_Evaluate(t *testing.T) {
	createTimestampExtractMock := func() TimestampExtract {
		orgRepo := new(mocks.OrganizationRepository)
		execFac := &mocks.ExecutorFactory{}
		transaction := &mocks.Transaction{}
		orgId := "0193721e-88d9-7f67-9221-f7fbeb1a1e9e"
		te := NewTimestampExtract(execFac, orgRepo, orgId)
		execFac.On("NewExecutor").Once().Return(transaction)
		ctx := context.Background()
		orgRepo.On("GetOrganizationById", ctx, transaction, orgId).Return(models.Organization{
			Id:                      orgId,
			Name:                    "Name",
			DefaultScenarioTimezone: utils.Ptr("UTC"),
		}, nil)
		return te
	}

	tests := []struct {
		name      string
		timestamp time.Time
		part      string
		expected  any
		expectErr bool
	}{
		{
			name:      "Extract year",
			timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			part:      "year",
			expected:  2023,
			expectErr: false,
		},
		{
			name:      "Extract month",
			timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			part:      "month",
			expected:  1,
			expectErr: false,
		},
		{
			name:      "Extract day of month",
			timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			part:      "day_of_month",
			expected:  1,
			expectErr: false,
		},
		{
			name:      "Extract day of week",
			timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			part:      "day_of_week",
			expected:  7, // Sunday
			expectErr: false,
		},
		{
			name:      "Extract hour",
			timestamp: time.Date(2023, time.January, 1, 15, 0, 0, 0, time.UTC),
			part:      "hour",
			expected:  15,
			expectErr: false,
		},
		{
			name:      "Invalid part",
			timestamp: time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
			part:      "minute",
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := createTimestampExtractMock()
			args := ast.Arguments{
				NamedArgs: map[string]any{
					"timestamp": tt.timestamp,
					"part":      tt.part,
				},
			}
			result, errs := te.Evaluate(context.Background(), args)
			if tt.expectErr {
				assert.NotNil(t, errs)
			} else {
				assert.Nil(t, errs)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
