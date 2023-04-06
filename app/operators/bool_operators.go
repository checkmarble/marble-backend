package operators

import (
	"encoding/json"
	"fmt"
	"log"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func unmarshalOperatorBool(jsonBytes []byte) (OperatorBool, error) {

	log.Printf("unmarshalOperatorBool: %v", string(jsonBytes))

	// All operators follow the same schema
	var _op struct {
		OperatorType
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(jsonBytes, &_op); err != nil {
		return nil, fmt.Errorf("unable to unmarshal operator to intermediate type/data representation: %w", err)
	}

	// find operator in map
	opFunc, found := operatorFromType[_op.Type]
	if !found {
		return nil, fmt.Errorf("operator %s not registered", _op.Type)
	}

	// cast operator to OperatorBool
	op, ok := opFunc().(OperatorBool)
	if !ok {
		return nil, fmt.Errorf("operator %s could not be cast to OperatorBool", _op.Type)
	}

	// unmarshal operator
	if err := json.Unmarshal(_op.Data, op); err != nil {
		return nil, fmt.Errorf("operator %s could not be unmarshalled: %w", _op.Type, err)
	}

	return op, nil
}

// /////////////////////////////
// True
// /////////////////////////////
type True struct{}

func (t True) Eval(d DataAccessor) (bool, error) { return true, nil }

func (t True) Print() string { return "TRUE" }

// Marshal with added "Type" operator
func (t True) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OperatorType
		Data string `json:"data"`
	}{
		OperatorType: OperatorType{Type: "TRUE"},
		Data:         "",
	})
}

// register creation
func init() {
	operatorFromType["TRUE"] = func() Operator { return &True{} }
}

func (t *True) UnmarshalJSON(b []byte) error {
	log.Println("unmarshaling True")
	return nil
}

// /////////////////////////////
// False
// /////////////////////////////
type False struct{}

func (f False) Eval(d DataAccessor) (bool, error) { return false, nil }

func (f False) Print() string { return "FALSE" }

// Marshal with added "Type" operator
func (f False) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OperatorType
		Data string `json:"data"`
	}{
		OperatorType: OperatorType{Type: "FALSE"},
		Data:         "",
	})
}

// register creation
func init() {
	operatorFromType["FALSE"] = func() Operator { return &False{} }
}

func (f *False) UnmarshalJSON(b []byte) error {
	log.Println("unmarshaling False")
	return nil
}

// /////////////////////////////
// Eq
// /////////////////////////////
type EqBool struct{ Left, Right OperatorBool }

func (eq EqBool) Eval(d DataAccessor) (bool, error) {
	valLeft, errLeft := eq.Left.Eval(d)
	valRight, errRight := eq.Right.Eval(d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in EqBool.Eval: %v, %v", errLeft, errRight)
	}
	return valLeft == valRight, nil
}

func (eq EqBool) Print() string {
	return fmt.Sprintf("( %s =bool %s )", eq.Left.Print(), eq.Right.Print())
}

func (eq EqBool) MarshalJSON() ([]byte, error) {

	// data schema
	type eqData struct {
		LeftOp  OperatorBool `json:"left"`
		RightOp OperatorBool `json:"right"`
	}

	return json.Marshal(struct {
		OperatorType
		Data eqData `json:"data"`
	}{
		OperatorType: OperatorType{Type: "EQBOOL"},
		Data: eqData{
			LeftOp:  eq.Left,
			RightOp: eq.Right,
		},
	})
}

// register creation
func init() {
	operatorFromType["EQBOOL"] = func() Operator { return &EqBool{} }
}

func (eq *EqBool) UnmarshalJSON(b []byte) error {

	log.Println("unmarshalling EQBOOL")

	// data schema
	var eqData struct {
		LeftOp  json.RawMessage `json:"left"`
		RightOp json.RawMessage `json:"right"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate left/right representation: %w", err)
	}

	// Build concrete Left operand
	left, err := unmarshalOperatorBool(eqData.LeftOp)
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	eq.Left = left

	// Build concrete Right operand
	right, err := unmarshalOperatorBool(eqData.RightOp)
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	eq.Right = right

	return nil
}

// /////////////////////////////
// Db field Boolean
// /////////////////////////////
type DbFieldBool struct {
	Path      []string
	FieldName string
}

func (field DbFieldBool) Eval(d DataAccessor) (bool, error) {
	val, err := d.GetDbField(field.Path, field.FieldName)
	if err != nil {
		fmt.Printf("Error getting DB field: %v", err)
		return false, err
	}
	va, ok := val.(bool)
	if ok {
		return va, nil
	}
	return false, fmt.Errorf("DB field %s is not a boolean", field.FieldName)
}

func (field DbFieldBool) Print() string {
	return fmt.Sprintf("( Boolean field from DB: path %v, field name %s )", field.Path, field.FieldName)
}

func (field DbFieldBool) MarshalJSON() ([]byte, error) {

	// data schema
	type dbFieldBoolData struct {
		Path      []string `json:"path"`
		FieldName string   `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		Data dbFieldBoolData `json:"data"`
	}{
		OperatorType: OperatorType{Type: "DB_FIELD_BOOL"},
		Data: dbFieldBoolData{
			Path:      field.Path,
			FieldName: field.FieldName,
		},
	})
}

// register creation
func init() {
	operatorFromType["DB_FIELD_BOOL"] = func() Operator { return &DbFieldBool{} }
}

func (field *DbFieldBool) UnmarshalJSON(b []byte) error {

	log.Println("unmarshalling DB_FIELD_BOOL")

	// data schema
	var dbFieldBoolData struct {
		Path      []string `json:"path"`
		FieldName string   `json:"fieldName"`
	}

	if err := json.Unmarshal(b, &dbFieldBoolData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate path/fieldName representation: %w", err)
	}
	field.Path = dbFieldBoolData.Path
	field.FieldName = dbFieldBoolData.FieldName

	return nil
}
