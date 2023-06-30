package ast


// ======= CustomListAccess =======

var AttributeFuncCustomListAccess = struct {
	FuncAttributes
	ArgumentCustomListId string
}{
	FuncAttributes: FuncAttributes{
		DebugName: "FUNC_CUSTOM_LIST_ACCESS",
		AstName:   "CustomListAccess",
		NamedArguments: []string{
			"customListId",
		},
	},
	ArgumentCustomListId: "customListId",
}

func NewNodeCustomListAccess(customListId string) Node {
	return Node{Function: FUNC_CUSTOM_LIST_ACCESS}.
		AddNamedChild(AttributeFuncCustomListAccess.ArgumentCustomListId, NewNodeConstant(customListId))
}
