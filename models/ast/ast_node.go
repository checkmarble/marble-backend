package ast

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/hashstructure/v2"
)

type Node struct {
	Index int `hash:"ignore"`

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

func (node Node) Hash() uint64 {
	hash, _ := hashstructure.Hash(node, hashstructure.FormatV2, nil)

	return hash
}

// Cost calculates the weights of an AST subtree to reorder, when the parent is commutative,
// nodes to prioritize faster ones.
func (node Node) Cost() int {
	selfCost := 0
	childCost := 0

	if attrs, err := node.Function.Attributes(); err == nil {
		selfCost = attrs.Cost
	}

	for _, ch := range node.Children {
		childCost += ch.Cost()
	}
	for _, ch := range node.NamedChildren {
		childCost += ch.Cost()
	}

	return selfCost + childCost
}

func (node *Node) PrintForAgent() (string, error) {
	return node.ToHumanReadable(), nil
}

// ToHumanReadable converts the AST node to a human-readable format with indentation
// similar to mathematical/logical expressions with proper grouping
func (node *Node) ToHumanReadable() string {
	return node.toHumanReadableWithDepth(0)
}

// Depth is the current depth of the node in the AST tree and is used to indent the output
func (node *Node) toHumanReadableWithDepth(depth int) string {
	// Handle constants
	if node.Function == FUNC_CONSTANT {
		return fmt.Sprintf("%v", node.Constant)
	}

	// Get function attributes
	attrs, err := node.Function.Attributes()
	if err != nil {
		return fmt.Sprintf("UNKNOWN_FUNC(%v)", node.Function)
	}

	if len(node.NamedChildren) > 0 {
		return node.formatMixedChildrenFunction(attrs.AstName, depth)
	}

	if len(node.Children) > 0 {
		return node.formatOperatorFunction(attrs.AstName, depth)
	}

	// Handle functions with no children (like TimeNow)
	return attrs.AstName
}

// formatMixedChildrenFunction formats functions that have both named children and regular children
// like Aggregator, Filter, TimeAdd, etc.
func (node *Node) formatMixedChildrenFunction(funcName string, depth int) string {
	var allParams []string

	// Add named children parameters
	allParams = append(allParams, node.formatNamedChildrenParams(depth)...)

	// Add regular children as positional arguments
	allParams = append(allParams, node.formatRegularChildrenParams(depth)...)

	return fmt.Sprintf("%s(%s)", funcName, strings.Join(allParams, ", "))
}

// formatNamedChildrenParams formats named children as key: value parameters
func (node *Node) formatNamedChildrenParams(depth int) []string {
	var params []string

	// Sort named children for consistent output
	keys := make([]string, 0, len(node.NamedChildren))
	for k := range node.NamedChildren {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		child := node.NamedChildren[key]
		childStr := child.toHumanReadableWithDepth(depth + 1)

		attrs, err := child.Function.Attributes()
		if err != nil {
			// Fallback to using the already formatted string for unknown functions
			params = append(params, fmt.Sprintf("%s: %s", key, childStr))
			continue
		}
		// Special handling for lists (like filters in Aggregator)
		// Example: Aggregator(aggregator: COUNT, fieldName: object_id, filters: [Filter(fieldName: payment_type, operator: =, tableName: transaction, value: card)], tableName: transaction)
		if key == "filters" && attrs.AstName == "List" {
			var filterStrs []string
			for _, filterChild := range child.Children {
				filterStrs = append(filterStrs, filterChild.toHumanReadableWithDepth(depth+1))
			}
			params = append(params, fmt.Sprintf("%s: [%s]", key, strings.Join(filterStrs, ", ")))
		} else {
			params = append(params, fmt.Sprintf("%s: %s", key, childStr))
		}
	}

	return params
}

// formatRegularChildrenParams formats regular children as positional arguments
func (node *Node) formatRegularChildrenParams(depth int) []string {
	var params []string

	for _, child := range node.Children {
		params = append(params, child.toHumanReadableWithDepth(depth+1))
	}

	return params
}

// formatOperatorFunction formats binary/unary operators and logical operators
func (node *Node) formatOperatorFunction(funcName string, depth int) string {
	if len(node.Children) == 0 {
		return funcName
	}

	// Get formatted children
	childStrs := node.formatRegularChildrenParams(depth)

	// Special formatting for different operator types
	switch node.Function {
	case FUNC_AND, FUNC_OR:
		// Logical operators: use uppercase and add line breaks for readability
		// Indentation is managed here
		operator := strings.ToUpper(funcName)
		indentation := strings.Repeat("  ", depth+1)
		baseIndentation := strings.Repeat("  ", depth)
		// len(childStrs) == 0 is not possible
		// Don't add the operator if there is only one child
		if len(childStrs) == 1 {
			return fmt.Sprintf("(\n%s%s\n%s)", indentation, childStrs[0], baseIndentation)
		}
		// Add the operator between the children
		return fmt.Sprintf("(\n%s%s\n%s)", indentation,
			strings.Join(childStrs, fmt.Sprintf("\n%s%s\n%s", indentation, operator, indentation)), baseIndentation)
	case FUNC_ADD, FUNC_SUBTRACT, FUNC_MULTIPLY, FUNC_DIVIDE,
		FUNC_GREATER, FUNC_GREATER_OR_EQUAL, FUNC_LESS, FUNC_LESS_OR_EQUAL,
		FUNC_EQUAL, FUNC_NOT_EQUAL:
		// Add the operator between the children
		return fmt.Sprintf("(%s)", strings.Join(childStrs, fmt.Sprintf(" %s ", funcName)))
	default:
		// Other functions: use prefix notation
		return fmt.Sprintf("%s(%s)", funcName, strings.Join(childStrs, ", "))
	}
}
