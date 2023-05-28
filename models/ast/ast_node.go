package ast

type Node struct {
	// A node is a constant xOR a function
	Function Function
	Constant any

	Children      []Node
	NamedChildren map[string]Node
}

func (node Node) AddChild(child Node) Node {
	node.Children = append(node.Children, child)
	return node
}

func (node Node) AddNamedChild(name string, child Node) Node {
	if node.NamedChildren == nil {
		node.NamedChildren = make(map[string]Node)
	}
	node.NamedChildren[name] = child
	return node
}
