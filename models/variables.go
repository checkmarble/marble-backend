package models

type Variable struct {
	Name         string
	SqlTemplate  string
	ArgumentType DataType
	OutputType   DataType
}
