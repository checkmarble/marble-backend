package ast

import "errors"

var ErrWrongNumberOfArgument = errors.New("wrong number of arguments")
var ErrMissingNamedArgument = errors.New("missing named argument")
var ErrUnknownFunction = errors.New("unknown function")
