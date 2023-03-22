package app

type Node interface {
	Returns() DataType
	Eval(Payload) interface{}
	Print(Payload) string
}
