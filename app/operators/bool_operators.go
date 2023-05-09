package operators

import (
	"encoding/json"
	"fmt"
	"strings"

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
	}{
		OperatorType: OperatorType{Type: "TRUE"},
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
	}{
		OperatorType: OperatorType{Type: "FALSE"},
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

	return json.Marshal(struct {
		OperatorType
		Data []OperatorBool `json:"children"`
	}{
		OperatorType: OperatorType{Type: "EQUAL_BOOL"},
		Data: []OperatorBool{
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
		Data dbFieldBoolData `json:"staticData"`
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

// ///////////////////////////////////////////////////////////////////////////////////////
// Payload field Boolean
// ///////////////////////////////////////////////////////////////////////////////////////
type PayloadFieldBool struct {
	FieldName string
}

func (field PayloadFieldBool) Eval(d DataAccessor) (bool, error) {

	valRaw := d.GetPayloadField(field.FieldName)

	valPointer, ok := valRaw.(*bool)
	if !ok {
		return false, fmt.Errorf("Payload field %s is not a pointer to a boolean", field.FieldName)
	}
	if valPointer == nil {
		return false, fmt.Errorf("Payload field %s is null", field.FieldName)
	}
	return *valPointer, nil
}

func (field PayloadFieldBool) Print() string {
	return fmt.Sprintf("( Boolean field from Payload: %s )", field.FieldName)
}

func (field PayloadFieldBool) MarshalJSON() ([]byte, error) {

	// data schema
	type payloadFieldBoolData struct {
		FieldName string `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData payloadFieldBoolData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "PAYLOAD_FIELD_BOOL"},
		StaticData: payloadFieldBoolData{
			FieldName: field.FieldName,
		},
	})
}

// register creation
func init() {
	operatorFromType["PAYLOAD_FIELD_BOOL"] = func() Operator { return &PayloadFieldBool{} }
}

func (field *PayloadFieldBool) UnmarshalJSON(b []byte) error {
	// data schema
	var dbFieldBoolData struct {
		StaticData struct {
			FieldName string `json:"fieldName"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &dbFieldBoolData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.FieldName = dbFieldBoolData.StaticData.FieldName

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// AND
// ///////////////////////////////////////////////////////////////////////////////////////

type And struct{ Operands []OperatorBool }

func (and And) Eval(d DataAccessor) (bool, error) {
	for _, op := range and.Operands {
		res, err := op.Eval(d)
		if err != nil {
			return false, err
		} else if !res {
			return false, nil
		}
	}
	return true, nil
}

func (and And) Print() string {
	opsPrinted := make([]string, len(and.Operands))
	for i, op := range and.Operands {
		opsPrinted[i] = op.Print()
	}
	return fmt.Sprintf("( %s )", strings.Join(opsPrinted, " AND "))
}

func (and And) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorBool `json:"children"`
	}{
		OperatorType: OperatorType{Type: "AND"},
		Children:     and.Operands,
	})
}

// register creation
func init() {
	operatorFromType["AND"] = func() Operator { return &And{} }
}

func (and *And) UnmarshalJSON(b []byte) error {
	// data schema
	var andData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &andData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(andData.Children) == 0 {
		return fmt.Errorf("No children for operator AND: %d operands", len(andData.Children))
	}

	children := make([]OperatorBool, len(andData.Children))
	for i, child := range andData.Children {
		// Build concrete operand
		op, err := UnmarshalOperatorBool(child)
		if err != nil {
			return fmt.Errorf("unable to instantiate AND operand: %w", err)
		}
		children[i] = op
	}
	and.Operands = children

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// OR
// ///////////////////////////////////////////////////////////////////////////////////////

type Or struct{ Operands []OperatorBool }

func (or Or) Eval(d DataAccessor) (bool, error) {
	for _, op := range or.Operands {
		res, err := op.Eval(d)
		if err != nil {
			return false, err
		} else if res {
			return true, nil
		}
	}
	return false, nil
}

func (or Or) Print() string {
	opsPrinted := make([]string, len(or.Operands))
	for i, op := range or.Operands {
		opsPrinted[i] = op.Print()
	}
	return fmt.Sprintf("( %s )", strings.Join(opsPrinted, " OR "))
}

func (or Or) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorBool `json:"children"`
	}{
		OperatorType: OperatorType{Type: "OR"},
		Children:     or.Operands,
	})
}

// register creation
func init() {
	operatorFromType["OR"] = func() Operator { return &Or{} }
}

func (or *Or) UnmarshalJSON(b []byte) error {
	// data schema
	var orData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &orData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(orData.Children) == 0 {
		return fmt.Errorf("No children for operator OR: %d operands", len(orData.Children))
	}

	children := make([]OperatorBool, len(orData.Children))
	for i, child := range orData.Children {
		// Build concrete operand
		op, err := UnmarshalOperatorBool(child)
		if err != nil {
			return fmt.Errorf("unable to instantiate OR operand: %w", err)
		}
		children[i] = op
	}
	or.Operands = children

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// NOT
// ///////////////////////////////////////////////////////////////////////////////////////

type Not struct{ Child OperatorBool }

func (not Not) Eval(d DataAccessor) (bool, error) {
	res, err := not.Child.Eval(d)
	if err != nil {
		return false, err
	}
	return !res, nil
}

func (not Not) Print() string {
	return fmt.Sprintf("( !%s )", not.Child.Print())
}

func (not Not) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorBool `json:"children"`
	}{
		OperatorType: OperatorType{Type: "NOT"},
		Children:     []OperatorBool{not.Child},
	})
}

// register creation
func init() {
	operatorFromType["NOT"] = func() Operator { return &Not{} }
}

func (not *Not) UnmarshalJSON(b []byte) error {
	// data schema
	var notData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &notData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(notData.Children) != 1 {
		return fmt.Errorf("Incorrect number of children operators for operator NOT: %d operands", len(notData.Children))
	}

	// Build concrete operand
	op, err := UnmarshalOperatorBool(notData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate NOT operand: %w", err)
	}
	not.Child = op

	return nil
}
