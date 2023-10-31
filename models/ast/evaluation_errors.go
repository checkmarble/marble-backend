package ast

import "errors"

var ErrUndefinedFunction = errors.New("undefined function")
var ErrWrongNumberOfArgument = errors.New("wrong number of arguments")
var ErrMissingNamedArgument = errors.New("missing named argument")
var ErrArgumentMustBeIntOrFloat = errors.New("arguments must be an integer or a float")
var ErrArgumentMustBeIntFloatOrTime = errors.New("all arguments must be an integer, a float or a time")
var ErrArgumentMustBeInt = errors.New("arguments must be an integer")
var ErrArgumentMustBeString = errors.New("arguments must be a string")
var ErrArgumentMustBeBool = errors.New("arguments must be a boolean")
var ErrArgumentMustBeList = errors.New("arguments must be a list")
var ErrArgumentCantBeConvertedToDuration = errors.New("argument cant be converted to duration")
var ErrArgumentCantBeTime = errors.New("argument must be a time")
var ErrArgumentRequired = errors.New("argument is required")
var ErrArgumentInvalidType = errors.New("argument has an invalid type")
var ErrListNotFound = errors.New("list not found")
var ErrDatabaseAccessNotFound = errors.New("database access not found")
