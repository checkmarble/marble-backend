package operators

import (
	"context"
	"encoding/json"
	"fmt"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func UnmarshalOperatorStringList(jsonBytes []byte) (OperatorStringList, error) {
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

	// cast operator to OperatorString
	typedOp, ok := opGetterFunc().(OperatorStringList)
	if !ok {
		return nil, fmt.Errorf("operator %s could not be cast to OperatorStringList interface", _opType.Type)
	}

	// unmarshal operator
	if err := json.Unmarshal(jsonBytes, typedOp); err != nil {
		return nil, fmt.Errorf("operator %s could not be unmarshalled: %w", _opType.Type, err)
	}

	return typedOp, nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// StringListValue
// ///////////////////////////////////////////////////////////////////////////////////////
type StringListValue struct {
	Value []string
}

// register creation
func init() {
	operatorFromType["STRING_LIST_CONSTANT"] = func() Operator { return &StringListValue{} }
}

func (s StringListValue) Eval(ctx context.Context, d DataAccessor) ([]string, error) {
	return s.Value, nil
}

func (s StringListValue) IsValid() bool { return s.Value != nil }

func (s StringListValue) String() string { return fmt.Sprintf("%v", s.Value) }

// Marshal with added "Type" operator
func (s StringListValue) MarshalJSON() ([]byte, error) {
	type stringValueIntermediate struct {
		Value []string `json:"value"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData stringValueIntermediate `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "STRING_LIST_CONSTANT"},
		StaticData:   stringValueIntermediate{Value: s.Value},
	})
}

func (s *StringListValue) UnmarshalJSON(b []byte) error {
	// data schema
	var stringValueData struct {
		StaticData struct {
			Strings []string `json:"value"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &stringValueData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	s.Value = stringValueData.StaticData.Strings

	return nil
}
