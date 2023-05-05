package operators

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func UnmarshalOperatorBool(jsonBytes []byte) (OperatorBool, error) {
	// All operators follow the same schema

	var _opType OperatorType

	if err := json.Unmarshal(jsonBytes, &_opType); err != nil {
		return nil, fmt.Errorf("unable to unmarshal operator to intermediate type/data representation: %w", err)
	}

	// find operator in map
	opGetterFunc, found := operatorFromType[_opType.Type]
	if !found {
		return nil, fmt.Errorf("operator %s not registered", _opType.Type)
	}

	// cast operator to OperatorBool
	typedOp, ok := opGetterFunc().(OperatorBool)
	if !ok {
		return nil, fmt.Errorf("operator %s could not be cast to OperatorBool interface", _opType.Type)
	}

	// unmarshal operator
	if err := json.Unmarshal(jsonBytes, typedOp); err != nil {
		return nil, fmt.Errorf("operator %s could not be unmarshalled: %w", _opType.Type, err)
	}

	return typedOp, nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// True
// ///////////////////////////////////////////////////////////////////////////////////////
type True struct{}

func (t True) Eval(d DataAccessor) (bool, error) { return true, nil }

func (t True) Print() string { return "TRUE" }

// Marshal with added "Type" operator
func (t True) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OperatorType
		Children   []Operator  `json:"children"`
		StaticData interface{} `json:"static_data"`
	}{
		OperatorType: OperatorType{Type: "TRUE"},
		Children:     []Operator{},
	})
}

// register creation
func init() {
	operatorFromType["TRUE"] = func() Operator { return &True{} }
}

func (t True) UnmarshalJSON(b []byte) error {
	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// False
// ///////////////////////////////////////////////////////////////////////////////////////
type False struct{}

func (f False) Eval(d DataAccessor) (bool, error) { return false, nil }

func (f False) Print() string { return "FALSE" }

// Marshal with added "Type" operator
func (f False) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OperatorType
		Children   []Operator  `json:"children"`
		StaticData interface{} `json:"static_data"`
	}{
		OperatorType: OperatorType{Type: "FALSE"},
		Children:     []Operator{},
	})
}

// register creation
func init() {
	operatorFromType["FALSE"] = func() Operator { return &False{} }
}

func (f False) UnmarshalJSON(b []byte) error {
	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Eq
// ///////////////////////////////////////////////////////////////////////////////////////
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
		Children   []OperatorBool `json:"children"`
		StaticData interface{}    `json:"static_data"`
	}{
		OperatorType: OperatorType{Type: "EQUAL_BOOL"},
		Children: []OperatorBool{
			eq.Left,
			eq.Right,
		},
	})
}

// register creation
func init() {
	operatorFromType["EQUAL_BOOL"] = func() Operator { return &EqBool{} }
}

func (eq *EqBool) UnmarshalJSON(b []byte) error {
	// data schema
	var eqData struct {
		Children []json.RawMessage `json:"children"`
	}

	if err := json.Unmarshal(b, &eqData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(eqData.Children) != 2 {
		return fmt.Errorf("wrong number of children for operator EQUAL_BOOL: %d", len(eqData.Children))
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorBool(eqData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	eq.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorBool(eqData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	eq.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Db field Boolean
// ///////////////////////////////////////////////////////////////////////////////////////
type DbFieldBool struct {
	Path      []string
	FieldName string
}

func (field DbFieldBool) Eval(d DataAccessor) (bool, error) {
	err := d.ValidateDbFieldReadConsistency(field.Path, field.FieldName)
	if err != nil {
		return false, err
	}

	valRaw, err := d.GetDbField(field.Path, field.FieldName)
	if err != nil {
		fmt.Printf("Error getting DB field: %v", err)
		return false, err
	}

	valNullable, ok := valRaw.(pgtype.Bool)
	if !ok {
		return false, fmt.Errorf("DB field %s is not a boolean", field.FieldName)
	}
	if !valNullable.Valid {
		return false, fmt.Errorf("DB field %s is null", field.FieldName)
	}
	return valNullable.Bool, nil
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
		Children   []OperatorBool `json:"children"`
		StaticData interface{}    `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "DB_FIELD_BOOL"},
		StaticData: dbFieldBoolData{
			Path:      field.Path,
			FieldName: field.FieldName,
		},
		Children: []OperatorBool{},
	})
}

// register creation
func init() {
	operatorFromType["DB_FIELD_BOOL"] = func() Operator { return &DbFieldBool{} }
}

func (field *DbFieldBool) UnmarshalJSON(b []byte) error {
	// data schema
	var dbFieldBoolData struct {
		StaticData struct {
			Path      []string `json:"path"`
			FieldName string   `json:"fieldName"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &dbFieldBoolData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.Path = dbFieldBoolData.StaticData.Path
	field.FieldName = dbFieldBoolData.StaticData.FieldName

	return nil
}
