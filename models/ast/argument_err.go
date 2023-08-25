package ast

import "fmt"

type ArgumentError struct {
	ArgumentIndex int
	ArgumentName  string
}

func (e ArgumentError) Error() string {
	if e.ArgumentIndex >= 0 {
		return fmt.Sprintf("argument: %d", e.ArgumentIndex)
	}
	return fmt.Sprintf("named argument: %s", e.ArgumentName)
}

func NewArgumentError(argumentIndex int) ArgumentError {
	return ArgumentError{
		ArgumentIndex: argumentIndex,
		ArgumentName:  "",
	}
}

func NewNamedArgumentError(argumentName string) ArgumentError {
	return ArgumentError{
		ArgumentIndex: -1,
		ArgumentName:  argumentName,
	}
}
