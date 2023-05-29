package ast

func NewAstCompareBalance() Node {
	// ReadPayload("balance") + 5 > 100
	return Node{Function: FUNC_GREATER}.
		AddChild(Node{Function: FUNC_PLUS}.
			AddChild(NewNodeReadPayload("balance")).
			AddChild(Node{Constant: 5}),
		).
		AddChild(Node{Constant: 100})
}
