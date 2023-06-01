package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func UnmarshalOperatorBool(jsonBytes []byte) (OperatorBool, error) {
	// All operators follow the same schema
	if string(jsonBytes) == "null" {
		return nil, nil
	}

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
// BoolValue
// ///////////////////////////////////////////////////////////////////////////////////////
type BoolValue struct {
	Value bool
}

// register creation
func init() {
	operatorFromType["BOOL_CONSTANT"] = func() Operator { return &BoolValue{} }
}

func (bv BoolValue) Eval(ctx context.Context, d DataAccessor) (bool, error) { return bv.Value, nil }

func (bv BoolValue) IsValid() bool { return true }

func (bv BoolValue) String() string { return fmt.Sprintf("%v", bv.Value) }

// Marshal with added "Type" operator
func (bv BoolValue) MarshalJSON() ([]byte, error) {
	type boolValueIntermediate struct {
		Value bool `json:"value"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData boolValueIntermediate `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "BOOL_CONSTANT"},
		StaticData:   boolValueIntermediate{bv.Value},
	})
}

func (bv *BoolValue) UnmarshalJSON(b []byte) error {
	// data schema
	var boolValueData struct {
		StaticData struct {
			Value bool `json:"value"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &boolValueData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	bv.Value = boolValueData.StaticData.Value

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Eq
// ///////////////////////////////////////////////////////////////////////////////////////
type EqBool struct{ Left, Right OperatorBool }

// register creation
func init() {
	operatorFromType["EQUAL_BOOL"] = func() Operator { return &EqBool{} }
}

func (eq EqBool) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !eq.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}
	valLeft, errLeft := eq.Left.Eval(ctx, d)
	valRight, errRight := eq.Right.Eval(ctx, d)
	if errLeft != nil || errRight != nil {
		return false, fmt.Errorf("error in EqBool.Eval: %w, %w", errLeft, errRight)
	}
	return valLeft == valRight, nil
}

func (eq EqBool) IsValid() bool {
	return eq.Left != nil && eq.Right != nil && eq.Left.IsValid() && eq.Right.IsValid()
}

func (eq EqBool) String() string {
	return fmt.Sprintf("( %s =bool %s )", eq.Left.String(), eq.Right.String())
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
	TriggerTableName string
	Path             []string
	FieldName        string
}

// register creation
func init() {
	operatorFromType["DB_FIELD_BOOL"] = func() Operator { return &DbFieldBool{} }
}

func (field DbFieldBool) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !field.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	valRaw, err := d.GetDbField(field.TriggerTableName, field.Path, field.FieldName)
	if err != nil {
		return false, err
	}

	valNullable, ok := valRaw.(pgtype.Bool)
	if !ok {
		return false, fmt.Errorf("DB field %s is not a boolean", field.FieldName)
	}
	if !valNullable.Valid {
		return false, fmt.Errorf("DB field %s is null: %w", field.FieldName, models.OperatorNullValueReadError)
	}
	return valNullable.Bool, nil
}

func (field DbFieldBool) IsValid() bool {
	return field.TriggerTableName != "" && len(field.Path) > 0 && field.FieldName != ""
}

func (field DbFieldBool) String() string {
	return fmt.Sprintf("( Boolean field from DB: path %v, field name %s )", field.Path, field.FieldName)
}

func (field DbFieldBool) MarshalJSON() ([]byte, error) {

	// data schema
	type dbFieldBoolData struct {
		TriggerTableName string   `json:"triggerTableName"`
		Path             []string `json:"path"`
		FieldName        string   `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		Data dbFieldBoolData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "DB_FIELD_BOOL"},
		Data: dbFieldBoolData{
			TriggerTableName: field.TriggerTableName,
			Path:             field.Path,
			FieldName:        field.FieldName,
		},
	})
}

func (field *DbFieldBool) UnmarshalJSON(b []byte) error {
	// data schema
	var dbFieldBoolData struct {
		StaticData struct {
			TriggerTableName string   `json:"triggerTableName"`
			Path             []string `json:"path"`
			FieldName        string   `json:"fieldName"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &dbFieldBoolData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.TriggerTableName = dbFieldBoolData.StaticData.TriggerTableName
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

// register creation
func init() {
	operatorFromType["PAYLOAD_FIELD_BOOL"] = func() Operator { return &PayloadFieldBool{} }
}

func (field PayloadFieldBool) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !field.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	return getPayloadFieldGeneric[bool](d, field.FieldName)
}

func (field PayloadFieldBool) IsValid() bool {
	return field.FieldName != ""
}

func (field PayloadFieldBool) String() string {
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

func (field *PayloadFieldBool) UnmarshalJSON(b []byte) error {
	// data schema
	var payloadFieldData struct {
		StaticData struct {
			FieldName string `json:"fieldName"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &payloadFieldData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.FieldName = payloadFieldData.StaticData.FieldName

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// AND
// ///////////////////////////////////////////////////////////////////////////////////////

type And struct{ Operands []OperatorBool }

// register creation
func init() {
	operatorFromType["AND"] = func() Operator { return &And{} }
}

func (and And) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !and.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	for _, op := range and.Operands {
		res, err := op.Eval(ctx, d)
		if err != nil {
			return false, err
		} else if !res {
			return false, nil
		}
	}
	return true, nil
}

func (and And) IsValid() bool {
	if len(and.Operands) == 0 {
		return false
	}
	for _, op := range and.Operands {
		if op == nil || !op.IsValid() {
			return false
		}
	}
	return true
}

func (and And) String() string {
	opsPrinted := make([]string, len(and.Operands))
	for i, op := range and.Operands {
		opsPrinted[i] = op.String()
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

func (and *And) UnmarshalJSON(b []byte) error {
	// data schema
	var andData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &andData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
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

// register creation
func init() {
	operatorFromType["OR"] = func() Operator { return &Or{} }
}

func (or Or) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !or.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	for _, op := range or.Operands {
		res, err := op.Eval(ctx, d)
		if err != nil {
			return false, err
		} else if res {
			return true, nil
		}
	}
	return false, nil
}

func (or Or) IsValid() bool {
	if len(or.Operands) == 0 {
		return false
	}
	for _, op := range or.Operands {
		if op == nil || !op.IsValid() {
			return false
		}
	}
	return true
}

func (or Or) String() string {
	opsPrinted := make([]string, len(or.Operands))
	for i, op := range or.Operands {
		opsPrinted[i] = op.String()
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

func (or *Or) UnmarshalJSON(b []byte) error {
	// data schema
	var orData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &orData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
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

// register creation
func init() {
	operatorFromType["NOT"] = func() Operator { return &Not{} }
}

func (not Not) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !not.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	res, err := not.Child.Eval(ctx, d)
	if err != nil {
		return false, err
	}
	return !res, nil
}

func (not Not) IsValid() bool {
	return not.Child != nil && not.Child.IsValid()
}

func (not Not) String() string {
	return fmt.Sprintf("( !%s )", not.Child.String())
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

// ///////////////////////////////////////////////////////////////////////////////////////
// String in list
// ///////////////////////////////////////////////////////////////////////////////////////

type StringIsInList struct {
	Str  OperatorString
	List OperatorStringList
}

// register creation
func init() {
	operatorFromType["STRING_IS_IN_LIST"] = func() Operator { return &StringIsInList{} }
}

func (s StringIsInList) Eval(ctx context.Context, d DataAccessor) (bool, error) {
	if !s.IsValid() {
		return false, ErrEvaluatingInvalidOperator
	}

	str, err := s.Str.Eval(ctx, d)
	if err != nil {
		return false, err
	}
	list, err := s.List.Eval(ctx, d)
	if err != nil {
		return false, err
	}
	for _, listItem := range list {
		if str == listItem {
			return true, nil
		}
	}
	return false, nil
}

func (s StringIsInList) IsValid() bool {
	return s.Str != nil && s.Str.IsValid() && s.List != nil && s.List.IsValid()
}

func (s StringIsInList) String() string {
	return fmt.Sprintf("( %s IN (%s) )", s.Str.String(), s.List.String())
}

func (s StringIsInList) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []Operator `json:"children"`
	}{
		OperatorType: OperatorType{Type: "STRING_IS_IN_LIST"},
		Children:     []Operator{s.Str, s.List},
	})
}

func (s *StringIsInList) UnmarshalJSON(b []byte) error {
	// data schema
	var notData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &notData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Check number of children
	if len(notData.Children) != 2 {
		return fmt.Errorf("Incorrect number of children operators for operator STRING IS IN LIST: %d operands", len(notData.Children))
	}

	// Build concrete operand
	str, err := UnmarshalOperatorString(notData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate string operand: %w", err)
	}
	s.Str = str

	// Build concrete operand
	list, err := UnmarshalOperatorStringList(notData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate string list operand: %w", err)
	}
	s.List = list

	return nil
}
