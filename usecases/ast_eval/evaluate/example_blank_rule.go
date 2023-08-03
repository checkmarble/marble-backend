package evaluate

import "marble/marble-backend/models/ast"

func ReturnExampleBlankRuleAstNode() ast.Node {
	node := ast.Node{
		Function: ast.FUNC_AND,
		Children: []ast.Node{
			{
				// Total value debited in the 10 days after the first transaction is greated than 1000...
				Function: ast.FUNC_GREATER,
				Children: []ast.Node{
					{
						Function: ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT,
						Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
							Children: []ast.Node{{Constant: "accountId"}},
						}},
						NamedChildren: map[string]ast.Node{
							"direction": {Constant: "Debit"},
							"created_from": {
								Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
								Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
									Children: []ast.Node{{Constant: "accountId"}},
								}},
							},
							"created_to": {
								Function: ast.FUNC_ADD_TIME,
								Children: []ast.Node{
									{
										Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
										Children: []ast.Node{
											{Function: ast.FUNC_PAYLOAD,
												Children: []ast.Node{{Constant: "accountId"}},
											},
										},
									},
									// can't express it in units larger than hours (normal, because "day" is not a proper duration)
									{Constant: "240h"},
								},
							},
						},
					},
					{Constant: 1000},
				},
			},
			// and the total value debited is larger than 90% of the value credited over that period
			{
				Function: ast.FUNC_GREATER,
				Children: []ast.Node{
					{
						Function: ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT,
						Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
							Children: []ast.Node{{Constant: "accountId"}},
						}},
						NamedChildren: map[string]ast.Node{
							"direction": {Constant: "Debit"},
							"created_from": {
								Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
								Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
									Children: []ast.Node{{Constant: "accountId"}},
								}},
							},
							"created_to": {
								Function: ast.FUNC_ADD_TIME,
								Children: []ast.Node{
									{
										Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
										Children: []ast.Node{
											{Function: ast.FUNC_PAYLOAD,
												Children: []ast.Node{{Constant: "accountId"}},
											},
										},
									},
									{Constant: "240h"},
								},
							},
						},
					},
					{
						Function: ast.FUNC_MULTIPLY,
						Children: []ast.Node{
							{
								Function: ast.FUNC_BLANK_SUM_TRANSACTIONS_AMOUNT,
								Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
									Children: []ast.Node{{Constant: "accountId"}},
								}},
								NamedChildren: map[string]ast.Node{
									"direction": {Constant: "Credit"},
									"created_from": {
										Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
										Children: []ast.Node{{Function: ast.FUNC_PAYLOAD,
											Children: []ast.Node{{Constant: "accountId"}},
										}},
									},
									"created_to": {
										Function: ast.FUNC_ADD_TIME,
										Children: []ast.Node{
											{
												Function: ast.FUNC_BLANK_FIRST_TRANSACTION_DATE,
												Children: []ast.Node{
													{Function: ast.FUNC_PAYLOAD,
														Children: []ast.Node{{Constant: "accountId"}},
													},
												},
											},
											{Constant: "240h"},
										},
									},
								},
							},
							{Constant: 0.9},
						},
					},
				},
			},
		},
	}
	return node
}

/*
Or in pseudo code:
(
"sum value transaction" (direction Debit, from "first transaction date", to "first transaction date" + 10 days) > 1000
) AND (
"sum value transaction" (direction Debit, from "first transaction date", to "first transaction date" + 10 days)
	> 90% * "sum value transaction" (direction Credit, from "first transaction date", to "first transaction date" + 10 days)
)
*/
