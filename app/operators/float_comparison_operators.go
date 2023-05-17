package operators

import (
	"encoding/json"
	"fmt"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// Greater or equal float
// ///////////////////////////////////////////////////////////////////////////////////////
type GreaterFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["GREATER_FLOAT"] = func() Operator { return &GreaterFloat{} }
}

func (o GreaterFloat) Eval(d DataAccessor) (bool, error) {
	if !o.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := o.Left.Eval(d)
	valRight, errRight := o.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in GreaterFloat.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft > valRight, nil
}

func (o GreaterFloat) IsValid() bool {
	return o.Left != nil && o.Right != nil && o.Left.IsValid() && o.Right.IsValid()
}

func (o GreaterFloat) String() string {
	return fmt.Sprintf("( %s > (float) %s )", o.Left.String(), o.Right.String())
}

func (o GreaterFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "GREATER_FLOAT"},
		Data: []OperatorFloat{
			o.Left,
			o.Right,
		},
	})
}

func (o *GreaterFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator GREATER_FLOAT: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	o.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	o.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Greater or equal float
// ///////////////////////////////////////////////////////////////////////////////////////
type GreaterOrEqualFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["GREATER_OR_EQUAL_FLOAT"] = func() Operator { return &GreaterOrEqualFloat{} }
}

func (o GreaterOrEqualFloat) Eval(d DataAccessor) (bool, error) {
	if !o.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := o.Left.Eval(d)
	valRight, errRight := o.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in GreaterOrEqualFloat.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft >= valRight, nil
}

func (o GreaterOrEqualFloat) IsValid() bool {
	return o.Left != nil && o.Right != nil && o.Left.IsValid() && o.Right.IsValid()
}

func (o GreaterOrEqualFloat) String() string {
	return fmt.Sprintf("( %s >= (float) %s )", o.Left.String(), o.Right.String())
}

func (o GreaterOrEqualFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "GREATER_OR_EQUAL_FLOAT"},
		Data: []OperatorFloat{
			o.Left,
			o.Right,
		},
	})
}

func (o *GreaterOrEqualFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator GREATER_OR_EQUAL_FLOAT: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	o.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	o.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Equal float
// ///////////////////////////////////////////////////////////////////////////////////////
type EqualFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["EQUAL_FLOAT"] = func() Operator { return &EqualFloat{} }
}

func (o EqualFloat) Eval(d DataAccessor) (bool, error) {
	if !o.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := o.Left.Eval(d)
	valRight, errRight := o.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in EqualFloat.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft == valRight, nil
}

func (o EqualFloat) IsValid() bool {
	return o.Left != nil && o.Right != nil && o.Left.IsValid() && o.Right.IsValid()
}

func (o EqualFloat) String() string {
	return fmt.Sprintf("( %s == (float) %s )", o.Left.String(), o.Right.String())
}

func (o EqualFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "EQUAL_FLOAT"},
		Data: []OperatorFloat{
			o.Left,
			o.Right,
		},
	})
}

func (o *EqualFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator EQUAL_FLOAT: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	o.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	o.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Lesser or Equal float
// ///////////////////////////////////////////////////////////////////////////////////////
type LesserOrEqualFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["LESSER_OR_EQUAL_FLOAT"] = func() Operator { return &LesserOrEqualFloat{} }
}

func (o LesserOrEqualFloat) Eval(d DataAccessor) (bool, error) {
	if !o.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := o.Left.Eval(d)
	valRight, errRight := o.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in LesserOrEqualFloat.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft <= valRight, nil
}

func (o LesserOrEqualFloat) IsValid() bool {
	return o.Left != nil && o.Right != nil && o.Left.IsValid() && o.Right.IsValid()
}

func (o LesserOrEqualFloat) String() string {
	return fmt.Sprintf("( %s <= (float) %s )", o.Left.String(), o.Right.String())
}

func (o LesserOrEqualFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "LESSER_OR_EQUAL_FLOAT"},
		Data: []OperatorFloat{
			o.Left,
			o.Right,
		},
	})
}

func (o *LesserOrEqualFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator LESSER_OR_EQUAL_FLOAT: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	o.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	o.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Lesser float
// ///////////////////////////////////////////////////////////////////////////////////////
type LesserFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["LESSER_FLOAT"] = func() Operator { return &LesserFloat{} }
}

func (o LesserFloat) Eval(d DataAccessor) (bool, error) {
	if !o.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := o.Left.Eval(d)
	valRight, errRight := o.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in LesserFloat.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft <= valRight, nil
}

func (o LesserFloat) IsValid() bool {
	return o.Left != nil && o.Right != nil && o.Left.IsValid() && o.Right.IsValid()
}

func (o LesserFloat) String() string {
	return fmt.Sprintf("( %s < (float) %s )", o.Left.String(), o.Right.String())
}

func (o LesserFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "LESSER_FLOAT"},
		Data: []OperatorFloat{
			o.Left,
			o.Right,
		},
	})
}

func (o *LesserFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator LESSER_FLOAT: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	o.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	o.Right = right

	return nil
}
