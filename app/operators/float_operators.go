package operators

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func UnmarshalOperatorFloat(jsonBytes []byte) (OperatorFloat, error) {
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

	// cast operator to OperatorFloat
	typedOp, ok := opGetterFunc().(OperatorFloat)
	if !ok {
		return nil, fmt.Errorf("operator %s could not be cast to OperatorFloat interface", _opType.Type)
	}

	// unmarshal operator
	if err := json.Unmarshal(jsonBytes, typedOp); err != nil {
		return nil, fmt.Errorf("operator %s could not be unmarshalled: %w", _opType.Type, err)
	}

	return typedOp, nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// FloatValue
// ///////////////////////////////////////////////////////////////////////////////////////
type FloatValue struct {
	Value float64
}

// register creation
func init() {
	operatorFromType["FLOAT_SCALAR"] = func() Operator { return &FloatValue{} }
}

func (f FloatValue) Eval(d DataAccessor) (float64, error) { return f.Value, nil }

func (f FloatValue) IsValid() bool { return true }

func (f FloatValue) String() string { return fmt.Sprintf("%f", f.Value) }

// Marshal with added "Type" operator
func (f FloatValue) MarshalJSON() ([]byte, error) {
	type floatValueIntermediate struct {
		Value float64 `json:"value"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData floatValueIntermediate `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "FLOAT_SCALAR"},
		StaticData:   floatValueIntermediate{f.Value},
	})
}

func (f *FloatValue) UnmarshalJSON(b []byte) error {
	// data schema
	var floatValueData struct {
		StaticData struct {
			Value float64 `json:"value"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &floatValueData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	f.Value = floatValueData.StaticData.Value

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Payload field Float
// ///////////////////////////////////////////////////////////////////////////////////////
type PayloadFieldFloat struct {
	FieldName string
}

// register creation
func init() {
	operatorFromType["PAYLOAD_FIELD_FLOAT"] = func() Operator { return &PayloadFieldBool{} }
}

func (field PayloadFieldFloat) Eval(d DataAccessor) (float64, error) {
	if !field.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	valRaw := d.GetPayloadField(field.FieldName)

	valPointer, ok := valRaw.(*float64)
	if !ok {
		return 0, fmt.Errorf("Payload field %s is not a pointer to a float", field.FieldName)
	}
	if valPointer == nil {
		return 0, fmt.Errorf("Payload field %s is null", field.FieldName)
	}
	return *valPointer, nil
}

func (field PayloadFieldFloat) IsValid() bool {
	return field.FieldName != ""
}

func (field PayloadFieldFloat) String() string {
	return fmt.Sprintf("( Float field from Payload: %s )", field.FieldName)
}

func (field PayloadFieldFloat) MarshalJSON() ([]byte, error) {

	// data schema
	type payloadFieldData struct {
		FieldName string `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData payloadFieldData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "PAYLOAD_FIELD_FLOAT"},
		StaticData: payloadFieldData{
			FieldName: field.FieldName,
		},
	})
}

func (field *PayloadFieldFloat) UnmarshalJSON(b []byte) error {
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
// SUM FLOAT
// ///////////////////////////////////////////////////////////////////////////////////////

type SumFloat struct{ Operands []OperatorFloat }

// register creation
func init() {
	operatorFromType["SUM_FLOAT"] = func() Operator { return &SumFloat{} }
}

func (s SumFloat) Eval(d DataAccessor) (float64, error) {
	if !s.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	total := 0.
	for _, op := range s.Operands {
		res, err := op.Eval(d)
		if err != nil {
			return 0, err
		} else {
			total += res
		}
	}
	return total, nil
}

func (s SumFloat) IsValid() bool {
	if len(s.Operands) == 0 {
		return false
	}
	for _, op := range s.Operands {
		if op == nil || !op.IsValid() {
			return false
		}
	}
	return true
}

func (s SumFloat) String() string {
	opsPrinted := make([]string, len(s.Operands))
	for i, op := range s.Operands {
		opsPrinted[i] = op.String()
	}
	return fmt.Sprintf("( %s )", strings.Join(opsPrinted, " + "))
}

func (s SumFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "SUM_FLOAT"},
		Children:     s.Operands,
	})
}

func (s *SumFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var sumData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &sumData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	children := make([]OperatorFloat, len(sumData.Children))
	for i, child := range sumData.Children {
		// Build concrete operand
		op, err := UnmarshalOperatorFloat(child)
		if err != nil {
			return fmt.Errorf("unable to instantiate SUM operand: %w", err)
		}
		children[i] = op
	}
	s.Operands = children

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// PRODUCT FLOAT
// ///////////////////////////////////////////////////////////////////////////////////////

type ProductFloat struct{ Operands []OperatorFloat }

// register creation
func init() {
	operatorFromType["PRODUCT_FLOAT"] = func() Operator { return &ProductFloat{} }
}

func (p ProductFloat) Eval(d DataAccessor) (float64, error) {
	if !p.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	total := 1.
	for _, op := range p.Operands {
		res, err := op.Eval(d)
		if err != nil {
			return 0, err
		} else {
			total *= res
		}
	}
	return total, nil
}

func (p ProductFloat) IsValid() bool {
	if len(p.Operands) == 0 {
		return false
	}
	for _, op := range p.Operands {
		if op == nil || !op.IsValid() {
			return false
		}
	}
	return true
}

func (p ProductFloat) String() string {
	opsPrinted := make([]string, len(p.Operands))
	for i, op := range p.Operands {
		opsPrinted[i] = op.String()
	}
	return fmt.Sprintf("( %s )", strings.Join(opsPrinted, " * "))
}

func (p ProductFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "PRODUCT_FLOAT"},
		Children:     p.Operands,
	})
}

func (p *ProductFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var productData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &productData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	children := make([]OperatorFloat, len(productData.Children))
	for i, child := range productData.Children {
		// Build concrete operand
		op, err := UnmarshalOperatorFloat(child)
		if err != nil {
			return fmt.Errorf("unable to instantiate product operand: %w", err)
		}
		children[i] = op
	}
	p.Operands = children

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// SUBTRACT FLOAT
// ///////////////////////////////////////////////////////////////////////////////////////

type SubtractFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["SUBTRACT_FLOAT"] = func() Operator { return &SubtractFloat{} }
}

func (s SubtractFloat) Eval(d DataAccessor) (float64, error) {
	if !s.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	left, err := s.Left.Eval(d)
	if err != nil {
		return 0, err
	}
	right, err := s.Right.Eval(d)
	if err != nil {
		return 0, err
	}
	return left - right, nil
}

func (s SubtractFloat) IsValid() bool {
	return s.Left != nil && s.Right != nil && s.Left.IsValid() && s.Right.IsValid()
}

func (s SubtractFloat) String() string {
	return fmt.Sprintf("( %s - %s )", s.Left.String(), s.Right.String())
}

func (s SubtractFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "SUBTRACT_FLOAT"},
		Children:     []OperatorFloat{s.Left, s.Right},
	})
}

func (s *SubtractFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var subtractData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &subtractData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(subtractData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	s.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(subtractData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	s.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// DIVIDE FLOAT
// ///////////////////////////////////////////////////////////////////////////////////////

type DivideFloat struct{ Left, Right OperatorFloat }

// register creation
func init() {
	operatorFromType["DIVIDE_FLOAT"] = func() Operator { return &DivideFloat{} }
}

func (div DivideFloat) Eval(d DataAccessor) (float64, error) {
	if !div.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	left, err := div.Left.Eval(d)
	if err != nil {
		return 0, err
	}
	right, err := div.Right.Eval(d)
	if err != nil {
		return 0, err
	} else if right == 0 {
		return 0, fmt.Errorf("Division by 0 error: %s", div.String())
	}
	return left / right, nil
}

func (div DivideFloat) IsValid() bool {
	return div.Left != nil && div.Right != nil && div.Left.IsValid() && div.Right.IsValid()
}

func (div DivideFloat) String() string {
	return fmt.Sprintf("( %s / %s )", div.Left.String(), div.Right.String())
}

func (div DivideFloat) MarshalJSON() ([]byte, error) {

	return json.Marshal(struct {
		OperatorType
		Children []OperatorFloat `json:"children"`
	}{
		OperatorType: OperatorType{Type: "DIVIDE_FLOAT"},
		Children:     []OperatorFloat{div.Left, div.Right},
	})
}

func (div *DivideFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var divideData struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal(b, &divideData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Build concrete Left operand
	left, err := UnmarshalOperatorFloat(divideData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate Left operator: %w", err)
	}
	div.Left = left

	// Build concrete Right operand
	right, err := UnmarshalOperatorFloat(divideData.Children[1])
	if err != nil {
		return fmt.Errorf("unable to instantiate Right operator: %w", err)
	}
	div.Right = right

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// ROUND FLOAT
// ///////////////////////////////////////////////////////////////////////////////////////

type RoundFloat struct {
	operand OperatorFloat
	level   int
}

// register creation
func init() {
	operatorFromType["ROUND_FLOAT"] = func() Operator { return &RoundFloat{} }
}

func (r RoundFloat) Eval(d DataAccessor) (float64, error) {
	if !r.IsValid() {
		return 0, ErrEvaluatingInvalidOperator
	}

	val, err := r.operand.Eval(d)
	if err != nil {
		return 0, err
	}

	ratio := math.Pow(10, float64(r.level))
	return math.Round(val*ratio) / ratio, nil
}

func (r RoundFloat) IsValid() bool {
	return r.operand != nil && r.operand.IsValid()
}

func (r RoundFloat) String() string {
	return fmt.Sprintf("ROUND(%s, %v)", r.operand.String(), r.level)
}

func (r RoundFloat) MarshalJSON() ([]byte, error) {
	type roundData struct {
		Level int `json:"level"`
	}

	return json.Marshal(struct {
		OperatorType
		Children   []OperatorFloat `json:"children"`
		StaticData roundData       `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "ROUND_FLOAT"},
		Children:     []OperatorFloat{r.operand},
		StaticData:   roundData{Level: r.level},
	})
}

func (r *RoundFloat) UnmarshalJSON(b []byte) error {
	// data schema
	var roundData struct {
		Children   []json.RawMessage `json:"children"`
		StaticData struct {
			Level int `json:"level"`
		} `json:"staticData"`
	}
	if err := json.Unmarshal(b, &roundData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Build concrete child operand
	child, err := UnmarshalOperatorFloat(roundData.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate child operator: %w", err)
	}
	r.operand = child
	r.level = roundData.StaticData.Level

	return nil
}
