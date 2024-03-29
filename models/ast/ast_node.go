package ast

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

type Node struct {
	// A node is a constant xOR a function
	Function Function
	Constant any

	Children      []Node
	NamedChildren map[string]Node
}

func (node *Node) DebugString() string {
	childrenDebugString := fmt.Sprintf("with %d children",
		len(node.Children)+len(node.NamedChildren))
	if node.Function == FUNC_CONSTANT {
		return fmt.Sprintf("Node Constant %v %s", node.Constant, childrenDebugString)
	}

	return fmt.Sprintf("Node %s %s", node.Function.DebugString(), childrenDebugString)
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

func (node Node) ReadConstantNamedChildString(name string) (string, error) {
	child, ok := node.NamedChildren[name]
	if !ok {
		return "", errors.New(fmt.Sprintf("Node does not have a %s child", name))
	}
	value, ok := child.Constant.(string)
	if !ok {
		return "", errors.New(fmt.Sprintf("\"%s\" constant is not a string: takes value %v", name, child.Constant))
	}
	return value, nil
}
