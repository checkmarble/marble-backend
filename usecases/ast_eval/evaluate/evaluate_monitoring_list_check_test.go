package evaluate

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidateMonitoringListCheckConfig(t *testing.T) {
	tests := []struct {
		name             string
		config           ast.MonitoringListCheckConfig
		expectedErrorLen int
		expectedErrors   []error
	}{
		{
			name: "happy path with LinkToSingleName",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "users",
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName:        "accounts",
						LinkToSingleName: utils.Ptr("account_id"),
					},
				},
			},
			expectedErrorLen: 0,
		},
		{
			name: "happy path with NavigationOption",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "users",
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName: "accounts",
						NavigationOption: &ast.NavigationOption{
							SourceTableName:   "source",
							SourceFieldName:   "source_id",
							TargetTableName:   "target",
							TargetFieldName:   "target_id",
							OrderingFieldName: "updated_at",
						},
					},
				},
			},
			expectedErrorLen: 0,
		},
		{
			name: "failure case with multiple validation errors",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "", // missing
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName:        "", // missing
						LinkToSingleName: nil,
						NavigationOption: nil, // neither present
					},
					{
						TableName:        "accounts",
						LinkToSingleName: utils.Ptr("account_id"),
						NavigationOption: &ast.NavigationOption{ // both present
							SourceTableName:   "source",
							SourceFieldName:   "source_id",
							TargetTableName:   "target",
							TargetFieldName:   "target_id",
							OrderingFieldName: "updated_at",
						},
					},
					{
						TableName: "accounts",
						NavigationOption: &ast.NavigationOption{ // missing all fields
							SourceTableName:   "",
							SourceFieldName:   "",
							TargetTableName:   "",
							TargetFieldName:   "",
							OrderingFieldName: "",
						},
					},
				},
			},
			expectedErrorLen: 9,
			expectedErrors: []error{
				ast.ErrArgumentRequired,    // targetTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[0].tableName
				ast.ErrArgumentRequired,    // linkedTableChecks[0]: neither link nor nav
				ast.ErrArgumentInvalidType, // linkedTableChecks[1]: both link and nav
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.targetTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.targetFieldName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.sourceTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.sourceFieldName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.orderingFieldName
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mlc := MonitoringListCheck{
				OrgId: uuid.New(),
			}

			errs := mlc.validateMonitoringListCheckConfig(tt.config)

			assert.Len(t, errs, tt.expectedErrorLen)
			for i, expectedErr := range tt.expectedErrors {
				assert.ErrorIs(t, errs[i], expectedErr)
			}
		})
	}
}
