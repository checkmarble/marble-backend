package operators

import (
	"encoding/json"
	"fmt"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// Eq string
// ///////////////////////////////////////////////////////////////////////////////////////
type EqString struct{ Left, Right OperatorString }

// register creation
func init() {
	operatorFromType["EQUAL_STRING"] = func() Operator { return &EqString{} }
}

func (eq EqString) Eval(d DataAccessor) (bool, error) {
	if !eq.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := eq.Left.Eval(d)
	valRight, errRight := eq.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in EqString.Eval: %w, %w", errLeft, errRight)
	}
	return valLeft == valRight, nil
}

func (eq EqString) IsValid() bool {
	return eq.Left != nil && eq.Right != nil && eq.Left.IsValid() && eq.Right.IsValid()
}

func (eq EqString) String() string {
	return fmt.Sprintf("( %s =string %s )", eq.Left.String(), eq.Right.String())
}

func (eq EqString) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Data []OperatorString `json:"children"`
	}{
		OperatorType: OperatorType{Type: "EQUAL_STRING"},
		Data: []OperatorString{
			eq.Left,
			eq.Right,
		},
	})
}

func (eq *EqString) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator EQUAL_STRING: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorString(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	eq.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorString(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	eq.Right = right

	return nil
}
