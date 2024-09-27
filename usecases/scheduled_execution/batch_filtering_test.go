package scheduled_execution

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func TestFilterFromComparisonNode(t *testing.T) {
	table := models.TableIdentifier{
		Schema: "schema",
		Table:  "table",
	}

	type testCase struct {
		name     string
		ruleJson string
		expected models.Filter
		valid    bool
	}

	testCases := []testCase{
		{
			name: "timestamp comparison with date shift",
			ruleJson: `{
				"name": "\u003c",
				"children": [
					{
					"name": "TimeAdd",
					"named_children": {
						"duration": { "constant": "PT0S" },
						"sign": { "constant": "+" },
						"timestampField": { "name": "TimeNow" }
					}
					},
					{ "name": "Payload", "children": [{ "constant": "updated_at" }] }
				]
			}`,
			expected: models.Filter{
				LeftSql:  "now() + interval 'PT0S'",
				Operator: ast.FUNC_LESS,
				RightSql: "\"schema\".\"table\".\"updated_at\"",
			},
			valid: true,
		},
		{
			name: "number comparison with nested division",
			ruleJson: `{
				"name": "\u003c",
				"children": [
					{ "constant": 1 },
					{
					"name": "/",
					"children": [
						{ "constant": 3 },
						{ "name": "Payload", "children": [{ "constant": "num" }] }
					]
					}
				]
			}`,
			expected: models.Filter{
				LeftValue: float64(1),
				Operator:  ast.FUNC_LESS,
				RightNestedFilter: &models.Filter{
					LeftValue: float64(3),
					Operator:  ast.FUNC_DIVIDE,
					RightSql:  "\"schema\".\"table\".\"num\"",
				},
			},
			valid: true,
		},
		{
			ruleJson: `{
				"name": "IsInList",
				"children": [
					{ "constant": "blabla" },
					{ "constant": ["bla", "and bla"] }
				]
			}`,
			expected: models.Filter{
				LeftValue:  "blabla",
				Operator:   ast.FUNC_IS_IN_LIST,
				RightValue: []string{"bla", "and bla"},
			},
			valid: true,
		},
		{
			name: "string contains comparison",
			ruleJson: `{
				"name": "StringContains",
				"children": [{ "constant": "COMMIT" }, { "constant": "mit" }]
			  }`,
			expected: models.Filter{
				LeftValue:  "COMMIT",
				Operator:   ast.FUNC_STRING_CONTAINS,
				RightValue: "mit",
			},
			valid: true,
		},
		{
			name: "db access",
			ruleJson: `{
				"name": "≠",
				"children": [
					{
					"name": "DatabaseAccess",
					"named_children": {
						"fieldName": { "constant": "new_table_pivot_field" },
						"path": { "constant": ["test_pivot_link"] },
						"tableName": { "constant": "Accounts" }
					}
					},
					{ "constant": "qzefzqef" }
				]
			}`,
			valid: false,
		},
		{
			name: "custom list access",
			ruleJson: `{
				"name": "IsInList",
				"children": [
					{ "name": "Payload", "children": [{ "constant": "account_name" }] },
					{
					"name": "CustomListAccess",
					"named_children": {
						"customListId": {
						"constant": "8a02e830-3094-4438-86e1-ab6762e778cf"
						}
					}
					}
				]
			}`,
			valid: false,
		},
		{
			name: "nested bool comparison",
			ruleJson: `{
				"name": "=",
				"children": [
					{ "name": "Payload", "children": [{ "constant": "after_migration" }] },
					{
					"name": "≠",
					"children": [
						{ "name": "Payload", "children": [{ "constant": "bool" }] },
						{ "constant": false }
					]
					}
				]
			}`,
			valid: true,
			expected: models.Filter{
				LeftSql:  "\"schema\".\"table\".\"after_migration\"",
				Operator: ast.FUNC_EQUAL,
				RightNestedFilter: &models.Filter{
					LeftSql:    "\"schema\".\"table\".\"bool\"",
					Operator:   ast.FUNC_NOT_EQUAL,
					RightValue: false,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var nodeDto dto.NodeDto
			err := json.Unmarshal([]byte(tc.ruleJson), &nodeDto)
			if err != nil {
				t.Fatal(err, tc.name)
			}
			node, err := dto.AdaptASTNode(nodeDto)
			if err != nil {
				t.Fatal(err, tc.name)
			}

			filter, valid := filterFromComparisonNode(node, table, 0)
			assert.Equal(t, tc.valid, valid, tc.name)
			assert.Equal(t, tc.expected, filter, tc.name)
		})
	}
}
