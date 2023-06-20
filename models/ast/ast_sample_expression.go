package ast

func NewAstCompareBalance() Node {
	// Variable("balance") + 5 > 100
	return Node{Function: FUNC_GREATER}.
		AddChild(Node{Function: FUNC_ADD}.
			AddChild(NewNodeVariable("balance")).
			AddChild(Node{Constant: 5}),
		).
		AddChild(Node{Constant: 100})
}
