package app

import (
	"fmt"
	"strings"
)

type And struct{ Left, Right Node }

func (a And) Returns() DataType { return Bool }
func (a And) Eval(p Payload) interface{} {
	return a.Left.Eval(p).(bool) && a.Right.Eval(p).(bool)
}
func (a And) Print(p Payload) string {
	return fmt.Sprintf("(%s AND %s)", a.Left.Print(p), a.Right.Print(p))
}

type True struct{}

func (t True) Returns() DataType          { return Bool }
func (t True) Eval(p Payload) interface{} { return true }
func (t True) Print(p Payload) string     { return "true" }

type False struct{}

func (f False) Returns() DataType          { return Bool }
func (f False) Eval(p Payload) interface{} { return false }
func (f False) Print(p Payload) string     { return "false" }

type Eq struct{ Left, Right Node }

func (eq Eq) Returns() DataType { return Bool }
func (eq Eq) Eval(p Payload) interface{} {
	return eq.Left.Eval(p) == eq.Right.Eval(p)
}
func (eq Eq) Print(p Payload) string {
	return fmt.Sprintf("(%s == %s)", eq.Left.Print(p), eq.Right.Print(p))
}

type IntValue struct{ Value int }

func (iv IntValue) Returns() DataType          { return Int }
func (iv IntValue) Eval(p Payload) interface{} { return iv.Value }
func (iv IntValue) Print(p Payload) string {
	return fmt.Sprintf("%v", iv.Value)
}

type FloatValue struct{ Value float64 }

func (fv FloatValue) Returns() DataType          { return Float }
func (fv FloatValue) Eval(p Payload) interface{} { return fv.Value }
func (fv FloatValue) Print(p Payload) string {
	return fmt.Sprintf("%.2f", fv.Value)
}

type FieldValue struct {
	Datamodel     DataModel
	RootTableName string
	Path          []string
}

func (fv FieldValue) Returns() DataType {
	return fv.Datamodel.FieldAt(fv.RootTableName, fv.Path).DataType
}

func (fv FieldValue) Eval(p Payload) interface{} {
	return fv.Datamodel.FieldValueAtFromPayload(p, fv.Path)
}
func (fv FieldValue) Print(p Payload) string {
	return fmt.Sprintf("%s.%s (%#v)", fv.RootTableName, strings.Join(fv.Path, "."), fv.Eval(p))
}
