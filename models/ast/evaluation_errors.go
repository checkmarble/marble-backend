package ast

import "github.com/cockroachdb/errors"

var (
	ErrUndefinedFunction                 = errors.New("undefined function")
	ErrWrongNumberOfArgument             = errors.New("wrong number of arguments")
	ErrMissingNamedArgument              = errors.New("missing named argument")
	ErrArgumentMustBeIntOrFloat          = errors.New("arguments must be an integer or a float")
	ErrArgumentMustBeIntFloatOrTime      = errors.New("all arguments must be an integer, a float or a time")
	ErrArgumentMustBeInt                 = errors.New("arguments must be an integer")
	ErrArgumentMustBeString              = errors.New("arguments must be a string")
	ErrArgumentMustBeBool                = errors.New("arguments must be a boolean")
	ErrArgumentMustBeList                = errors.New("arguments must be a list")
	ErrArgumentCantBeConvertedToDuration = errors.New("argument cant be converted to duration")
	ErrArgumentMustBeTime                = errors.New("argument must be a time")
	ErrArgumentRequired                  = errors.New("argument is required")
	ErrArgumentInvalidType               = errors.New("argument has an invalid type")
	ErrListNotFound                      = errors.New("list not found")
	ErrDatabaseAccessNotFound            = errors.New("database access not found")
)
