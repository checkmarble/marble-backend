package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToHumanReadable_Constants(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name:     "string constant",
			node:     NewNodeConstant("hello"),
			expected: "hello",
		},
		{
			name:     "integer constant",
			node:     NewNodeConstant(42),
			expected: "42",
		},
		{
			name:     "boolean constant",
			node:     NewNodeConstant(true),
			expected: "true",
		},
		{
			name:     "float constant",
			node:     NewNodeConstant(3.14),
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_NoChildrenFunctions(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name:     "TimeNow function",
			node:     Node{Function: FUNC_TIME_NOW},
			expected: "TimeNow",
		},
		{
			name:     "unknown function",
			node:     Node{Function: FUNC_UNDEFINED},
			expected: "Undefined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_BinaryOperators(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "simple addition",
			node: Node{
				Function: FUNC_ADD,
				Children: []Node{
					NewNodeConstant(5),
					NewNodeConstant(3),
				},
			},
			expected: "(5 + 3)",
		},
		{
			name: "greater than or equal",
			node: Node{
				Function: FUNC_GREATER_OR_EQUAL,
				Children: []Node{
					NewNodeConstant(10),
					NewNodeConstant(5),
				},
			},
			expected: "(10 >= 5)",
		},
		{
			name: "string equality",
			node: Node{
				Function: FUNC_EQUAL,
				Children: []Node{
					NewNodeConstant("card"),
					NewNodeConstant("payment_type"),
				},
			},
			expected: "(card = payment_type)",
		},
		{
			name: "division",
			node: Node{
				Function: FUNC_DIVIDE,
				Children: []Node{
					NewNodeConstant(100),
					NewNodeConstant(10),
				},
			},
			expected: "(100 / 10)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_LogicalOperators(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "simple AND",
			node: Node{
				Function: FUNC_AND,
				Children: []Node{
					NewNodeConstant(true),
					NewNodeConstant(false),
				},
			},
			expected: "(\n  true\n  AND\n  false\n)",
		},
		{
			name: "simple OR",
			node: Node{
				Function: FUNC_OR,
				Children: []Node{
					NewNodeConstant("a"),
					NewNodeConstant("b"),
				},
			},
			expected: "(\n  a\n  OR\n  b\n)",
		},
		{
			name: "single child AND",
			node: Node{
				Function: FUNC_AND,
				Children: []Node{
					NewNodeConstant("single"),
				},
			},
			expected: "(\n  single\n)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_NamedChildrenFunctions(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "TimeAdd function",
			node: Node{
				Function: FUNC_TIME_ADD,
				NamedChildren: map[string]Node{
					"timestampField": {Function: FUNC_TIME_NOW},
					"duration":       NewNodeConstant("P1D"),
					"sign":           NewNodeConstant("-"),
				},
			},
			expected: "TimeAdd(duration: P1D, sign: -, timestampField: TimeNow)",
		},
		{
			name: "Filter function",
			node: Node{
				Function: FUNC_FILTER,
				NamedChildren: map[string]Node{
					"tableName": NewNodeConstant("transaction"),
					"fieldName": NewNodeConstant("amount"),
					"operator":  NewNodeConstant(">="),
					"value":     NewNodeConstant(100),
				},
			},
			expected: "Filter(fieldName: amount, operator: >=, tableName: transaction, value: 100)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_ListsAndFilters(t *testing.T) {
	filterNode := Node{
		Function: FUNC_FILTER,
		NamedChildren: map[string]Node{
			"fieldName": NewNodeConstant("payment_type"),
			"operator":  NewNodeConstant("="),
			"tableName": NewNodeConstant("transaction"),
			"value":     NewNodeConstant("card"),
		},
	}

	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "Aggregator with filters",
			node: Node{
				Function: FUNC_AGGREGATOR,
				NamedChildren: map[string]Node{
					"aggregator": NewNodeConstant("COUNT"),
					"fieldName":  NewNodeConstant("object_id"),
					"tableName":  NewNodeConstant("transaction"),
					"filters": {
						Function: FUNC_LIST,
						Children: []Node{filterNode},
					},
				},
			},
			expected: "Aggregator(aggregator: COUNT, fieldName: object_id, filters: [Filter(fieldName: payment_type, operator: =, tableName: transaction, value: card)], tableName: transaction)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_MixedChildren(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "function with named and regular children",
			node: Node{
				Function: FUNC_STRING_TEMPLATE,
				NamedChildren: map[string]Node{
					"template": NewNodeConstant("Hello {0}"),
				},
				Children: []Node{
					NewNodeConstant("World"),
				},
			},
			expected: "StringTemplate(template: Hello {0}, World)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_NestedExpressions(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "nested arithmetic",
			node: Node{
				Function: FUNC_ADD,
				Children: []Node{
					{
						Function: FUNC_MULTIPLY,
						Children: []Node{
							NewNodeConstant(2),
							NewNodeConstant(3),
						},
					},
					NewNodeConstant(4),
				},
			},
			expected: "((2 * 3) + 4)",
		},
		{
			name: "deeply nested arithmetic with parentheses",
			node: Node{
				Function: FUNC_MULTIPLY,
				Children: []Node{
					{
						Function: FUNC_ADD,
						Children: []Node{
							NewNodeConstant(1),
							NewNodeConstant(2),
						},
					},
					{
						Function: FUNC_SUBTRACT,
						Children: []Node{
							NewNodeConstant(5),
							NewNodeConstant(3),
						},
					},
				},
			},
			expected: "((1 + 2) * (5 - 3))",
		},
		{
			name: "function with nested expression as argument",
			node: Node{
				Function: FUNC_IS_IN_LIST,
				Children: []Node{
					NewNodeConstant("value"),
					{
						Function: FUNC_ADD,
						Children: []Node{
							NewNodeConstant(1),
							NewNodeConstant(2),
						},
					},
				},
			},
			expected: "IsInList(value, (1 + 2))",
		},
		{
			name: "nested logical operators",
			node: Node{
				Function: FUNC_AND,
				Children: []Node{
					{
						Function: FUNC_EQUAL,
						Children: []Node{
							NewNodeConstant("payment_type"),
							NewNodeConstant("card"),
						},
					},
					{
						Function: FUNC_GREATER_OR_EQUAL,
						Children: []Node{
							NewNodeConstant("amount"),
							NewNodeConstant(100),
						},
					},
				},
			},
			expected: "(\n  (payment_type = card)\n  AND\n  (amount >= 100)\n)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_UnaryOperators(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "NOT operator",
			node: Node{
				Function: FUNC_NOT,
				Children: []Node{
					NewNodeConstant(true),
				},
			},
			expected: "Not(true)",
		},
		{
			name: "IsEmpty function",
			node: Node{
				Function: FUNC_IS_EMPTY,
				Children: []Node{
					NewNodeConstant("field_value"),
				},
			},
			expected: "IsEmpty(field_value)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToHumanReadable_ComplexRealWorldExample(t *testing.T) {
	// Create a very complex AST with 7+ conditions mixing OR and AND
	// This represents a sophisticated fraud detection rule

	// Condition 1: High transaction amount compared to historical average
	condition1 := Node{
		Function: FUNC_GREATER_OR_EQUAL,
		Children: []Node{
			{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("amount")}},
			{
				Function: FUNC_MULTIPLY,
				Children: []Node{
					NewNodeConstant(5),
					{
						Function: FUNC_AGGREGATOR,
						NamedChildren: map[string]Node{
							"aggregator": NewNodeConstant("AVG"),
							"fieldName":  NewNodeConstant("amount"),
							"tableName":  NewNodeConstant("transaction"),
							"filters": {
								Function: FUNC_LIST,
								Children: []Node{
									{
										Function: FUNC_FILTER,
										NamedChildren: map[string]Node{
											"fieldName": NewNodeConstant("account_holder_id"),
											"operator":  NewNodeConstant("="),
											"tableName": NewNodeConstant("transaction"),
											"value":     {Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("account_holder_id")}},
										},
									},
									{
										Function: FUNC_FILTER,
										NamedChildren: map[string]Node{
											"fieldName": NewNodeConstant("transaction_at"),
											"operator":  NewNodeConstant(">="),
											"tableName": NewNodeConstant("transaction"),
											"value": {
												Function: FUNC_TIME_ADD,
												NamedChildren: map[string]Node{
													"timestampField": {
														Function: FUNC_PAYLOAD,
														Children: []Node{NewNodeConstant("transaction_at")},
													},
													"duration": NewNodeConstant("P30D"),
													"sign":     NewNodeConstant("-"),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Condition 2: Transaction frequency spike
	condition2 := Node{
		Function: FUNC_GREATER,
		Children: []Node{
			{
				Function: FUNC_AGGREGATOR,
				NamedChildren: map[string]Node{
					"aggregator": NewNodeConstant("COUNT"),
					"fieldName":  NewNodeConstant("object_id"),
					"tableName":  NewNodeConstant("transaction"),
					"filters": {
						Function: FUNC_LIST,
						Children: []Node{
							{
								Function: FUNC_FILTER,
								NamedChildren: map[string]Node{
									"fieldName": NewNodeConstant("account_holder_id"),
									"operator":  NewNodeConstant("="),
									"tableName": NewNodeConstant("transaction"),
									"value":     {Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("account_holder_id")}},
								},
							},
							{
								Function: FUNC_FILTER,
								NamedChildren: map[string]Node{
									"fieldName": NewNodeConstant("transaction_at"),
									"operator":  NewNodeConstant(">="),
									"tableName": NewNodeConstant("transaction"),
									"value": {
										Function: FUNC_TIME_ADD,
										NamedChildren: map[string]Node{
											"timestampField": {Function: FUNC_TIME_NOW},
											"duration":       NewNodeConstant("PT1H"),
											"sign":           NewNodeConstant("-"),
										},
									},
								},
							},
						},
					},
				},
			},
			NewNodeConstant(10),
		},
	}

	// Condition 3: Merchant in suspicious list
	condition3 := Node{
		Function: FUNC_IS_IN_LIST,
		Children: []Node{
			{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("merchant_id")}},
			{
				Function: FUNC_CUSTOM_LIST_ACCESS,
				NamedChildren: map[string]Node{
					"customListId": NewNodeConstant("suspicious-merchants-001"),
				},
			},
		},
	}

	// Condition 4: Fuzzy match on merchant name
	condition4 := Node{
		Function: FUNC_FUZZY_MATCH_ANY_OF,
		NamedChildren: map[string]Node{
			"algorithm": NewNodeConstant("levenshtein"),
		},
		Children: []Node{
			{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("merchant_name")}},
			NewNodeConstant("suspicious_merchant"),
		},
	}

	// Condition 5: Late night transaction
	condition5 := Node{
		Function: FUNC_OR,
		Children: []Node{
			{
				Function: FUNC_LESS,
				Children: []Node{
					{
						Function: FUNC_TIMESTAMP_EXTRACT,
						NamedChildren: map[string]Node{
							"timestamp": {Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("transaction_at")}},
							"part":      NewNodeConstant("hour"),
						},
					},
					NewNodeConstant(6),
				},
			},
			{
				Function: FUNC_GREATER,
				Children: []Node{
					{
						Function: FUNC_TIMESTAMP_EXTRACT,
						NamedChildren: map[string]Node{
							"timestamp": {Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("transaction_at")}},
							"part":      NewNodeConstant("hour"),
						},
					},
					NewNodeConstant(22),
				},
			},
		},
	}

	// Condition 6: Arithmetic condition on transaction amount
	condition6 := Node{
		Function: FUNC_GREATER_OR_EQUAL,
		Children: []Node{
			{
				Function: FUNC_ADD,
				Children: []Node{
					{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("amount")}},
					{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("fees")}},
				},
			},
			{
				Function: FUNC_SUBTRACT,
				Children: []Node{
					NewNodeConstant(10000),
					{
						Function: FUNC_MULTIPLY,
						Children: []Node{
							{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("risk_score")}},
							NewNodeConstant(100),
						},
					},
				},
			},
		},
	}

	// Condition 7: Card country not in allowed list
	condition7 := Node{
		Function: FUNC_IS_NOT_IN_LIST,
		Children: []Node{
			{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("card_country")}},
			{
				Function: FUNC_CUSTOM_LIST_ACCESS,
				NamedChildren: map[string]Node{
					"customListId": NewNodeConstant("allowed-countries"),
				},
			},
		},
	}

	// Final complex rule: Multiple ORs and ANDs
	complexRule := Node{
		Function: FUNC_OR,
		Children: []Node{
			// First major branch: High risk profile
			{
				Function: FUNC_AND,
				Children: []Node{
					condition1, // High amount vs historical
					condition2, // High frequency
					condition5, // Late night
				},
			},
			// Second major branch: Merchant-based risk
			{
				Function: FUNC_AND,
				Children: []Node{
					{
						Function: FUNC_OR,
						Children: []Node{
							condition3, // Merchant in list
							condition4, // Fuzzy match merchant
						},
					},
					condition6, // High total amount
				},
			},
			// Third major branch: Geographic risk
			{
				Function: FUNC_AND,
				Children: []Node{
					condition7, // Country not allowed
					{
						Function: FUNC_GREATER,
						Children: []Node{
							{Function: FUNC_PAYLOAD, Children: []Node{NewNodeConstant("amount")}},
							NewNodeConstant(1000),
						},
					},
				},
			},
		},
	}

	result := complexRule.ToHumanReadable()
	expected := `(
  (
    (Payload(amount) >= (5 * Aggregator(aggregator: AVG, fieldName: amount, filters: [Filter(fieldName: account_holder_id, operator: =, tableName: transaction, value: Payload(account_holder_id)), Filter(fieldName: transaction_at, operator: >=, tableName: transaction, value: TimeAdd(duration: P30D, sign: -, timestampField: Payload(transaction_at)))], tableName: transaction)))
    AND
    (Aggregator(aggregator: COUNT, fieldName: object_id, filters: [Filter(fieldName: account_holder_id, operator: =, tableName: transaction, value: Payload(account_holder_id)), Filter(fieldName: transaction_at, operator: >=, tableName: transaction, value: TimeAdd(duration: PT1H, sign: -, timestampField: TimeNow))], tableName: transaction) > 10)
    AND
    (
      (TimestampExtract(part: hour, timestamp: Payload(transaction_at)) < 6)
      OR
      (TimestampExtract(part: hour, timestamp: Payload(transaction_at)) > 22)
    )
  )
  OR
  (
    (
      IsInList(Payload(merchant_id), CustomListAccess(customListId: suspicious-merchants-001))
      OR
      FuzzyMatchAnyOf(algorithm: levenshtein, Payload(merchant_name), suspicious_merchant)
    )
    AND
    ((Payload(amount) + Payload(fees)) >= (10000 - (Payload(risk_score) * 100)))
  )
  OR
  (
    IsNotInList(Payload(card_country), CustomListAccess(customListId: allowed-countries))
    AND
    (Payload(amount) > 1000)
  )
)`

	assert.Equal(t, expected, result)
}

func TestToHumanReadable_ErrorHandling(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "unknown function should fallback gracefully",
			node: Node{
				Function: FUNC_UNKNOWN,
				Children: []Node{
					NewNodeConstant("test"),
				},
			},
			expected: "UNKNOWN_FUNC(-2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.node.ToHumanReadable()
			assert.Equal(t, tt.expected, result)
		})
	}
}
