package operators

import (
	"context"
	"encoding/json"
	"fmt"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// IsInListOperator
// ///////////////////////////////////////////////////////////////////////////////////////
type IsNotInListBool struct {
	Left  OperatorString
	Right OperatorStringList
}

// register creation
func init() {
	operatorFromType["IS_NOT_IN_LIST_BOOL"] = func() Operator { return &IsNotInListBool{} }
}

func (lb IsNotInListBool) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !lb.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := lb.Left.Eval(ctx, d)
	valRight, errRight := lb.Right.Eval(ctx, d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in LbString.Eval: %w, %w", errLeft, errRight)
	}
	for _, val := range valRight {
		if val == valLeft {
			return false, nil
		}
	}
	return true, nil
}

func (lb IsNotInListBool) IsValid() bool {
	return lb.Left != nil && lb.Right != nil && lb.Left.IsValid() && lb.Right.IsValid()
}

func (lb IsNotInListBool) String() string {
	return fmt.Sprintf("( %s =string %s )", lb.Left.String(), lb.Right.String())
}

func (lb IsNotInListBool) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Left  OperatorString     `json:"left"`
		Right OperatorStringList `json:"right"`
	}{
		OperatorType: OperatorType{Type: "IS_NOT_IN_LIST_BOOL"},
		Left:         lb.Left,
		Right:        lb.Right,
	})
}

func (lb IsNotInListBool) UnmarshalJSON(b []byte) error {
	// data schema
	var lbData struct {
		Left json.RawMessage `json:"left"`
		Right json.RawMessage `json:"right"`
	}

	if err := json.Unmarshal(b, &lbData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(lbData.Left) != 2 {
		return fmt.Errorf("wrong number of children for operator IS_NOT_IN_LIST_BOOL: %d", len(lbData.Left))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorString(lbData.Left)
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	lb.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorStringList(lbData.Right)
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	lb.Right = right

	return nil
}
