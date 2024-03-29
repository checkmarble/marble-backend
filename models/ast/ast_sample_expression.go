package ast

func NewAstCompareBalance() Node {
	// (50 + 51) > 100
	return Node{Function: FUNC_GREATER}.
		AddChild(Node{Function: FUNC_ADD}.
			AddChild(Node{Constant: 51}).
			AddChild(Node{Constant: 50}),
		).
		AddChild(Node{Constant: 100})
}
