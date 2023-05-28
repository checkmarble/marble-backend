package ast

import "fmt"

type Arguments struct {
	Args      []any
	NamedArgs map[string]any
}

func (arguments *Arguments) GetNamedArgument(name string) (any, error) {
	if arg, ok := arguments.NamedArgs[name]; ok {
		return arg, nil
	}
	return nil, fmt.Errorf("named argument '%s' not found", name)
}

func (arguments *Arguments) StringNamedArgument(name string) (string, error) {

	value, err := arguments.GetNamedArgument(name)
	if err != nil {
		return "", err
	}

	if value, ok := value.(string); ok {
		return value, nil
	}
	return "", fmt.Errorf("named argument '%s' is not a string", name)
}
