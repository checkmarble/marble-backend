package ast

func NewAstCompareBalance() Node {
	return Node{Function: FUNC_GREATER}.
		AddChild(Node{Function: FUNC_ADD}.
			AddChild(Node{Constant: 51}).
			AddChild(Node{Constant: 50}),
		).
		AddChild(Node{Constant: 100})
}

func NewAstAndTrue() Node {
	return Node{Function: FUNC_AND}.
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true})
}

func NewAstAndFalse() Node {
	return Node{Function: FUNC_AND}.
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: true})
}


func NewAstOrTrue() Node {
	return Node{Function: FUNC_OR}.
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: true}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false})
}

func NewAstOrFalse() Node {
	return Node{Function: FUNC_OR}.
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false}).
		AddChild(Node{Constant: false})
}
