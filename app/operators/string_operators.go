package operators

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// /////////////////////////////
// get an unmarshalled operator
// /////////////////////////////

func UnmarshalOperatorString(jsonBytes []byte) (OperatorString, error) {
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
	typedOp, ok := opGetterFunc().(OperatorString)
	if !ok {
		return nil, fmt.Errorf("operator %s could not be cast to OperatorString interface", _opType.Type)
	}

	// unmarshal operator
	if err := json.Unmarshal(jsonBytes, typedOp); err != nil {
		return nil, fmt.Errorf("operator %s could not be unmarshalled: %w", _opType.Type, err)
	}

	return typedOp, nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// StringValue
// ///////////////////////////////////////////////////////////////////////////////////////
type StringValue struct {
	Text string
}

// register creation
func init() {
	operatorFromType["STRING_SCALAR"] = func() Operator { return &StringValue{} }
}

func (s StringValue) Eval(d DataAccessor) (string, error) { return s.Text, nil }

func (s StringValue) IsValid() bool { return true }

func (s StringValue) String() string { return s.Text }

// Marshal with added "Type" operator
func (s StringValue) MarshalJSON() ([]byte, error) {
	type stringValueIntermediate struct {
		Text string `json:"text"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData stringValueIntermediate `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "STRING_SCALAR"},
		StaticData:   stringValueIntermediate{s.Text},
	})
}

func (s *StringValue) UnmarshalJSON(b []byte) error {
	// data schema
	var stringValueData struct {
		StaticData struct {
			Text string `json:"text"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &stringValueData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	s.Text = stringValueData.StaticData.Text

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Db field Srting
// ///////////////////////////////////////////////////////////////////////////////////////
type DbFieldString struct {
	TriggerTableName string
	Path             []string
	FieldName        string
}

// register creation
func init() {
	operatorFromType["DB_FIELD_STRING"] = func() Operator { return &DbFieldString{} }
}

func (field DbFieldString) Eval(d DataAccessor) (string, error) {
	if !field.IsValid() {
		return "", ErrEvaluatingInvalidOperator
	}

	valRaw, err := d.GetDbField(field.TriggerTableName, field.Path, field.FieldName)
	if err != nil {
		fmt.Printf("Error getting DB field: %v", err)
		return "", err
	}

	valNullable, ok := valRaw.(pgtype.Text)
	if !ok {
		return "", fmt.Errorf("DB field %s is not a string", field.FieldName)
	}
	if !valNullable.Valid {
		return "", fmt.Errorf("DB field %s is null", field.FieldName)
	}
	return valNullable.String, nil
}

func (field DbFieldString) IsValid() bool {
	return field.TriggerTableName != "" && len(field.Path) > 0 && field.FieldName != ""
}

func (field DbFieldString) String() string {
	return fmt.Sprintf("( String field from DB: trigger %s, path %v, field name %s )", field.TriggerTableName, field.Path, field.FieldName)
}

func (field DbFieldString) MarshalJSON() ([]byte, error) {

	// data schema
	type dbFieldData struct {
		TriggerTableName string   `json:"triggerTableName"`
		Path             []string `json:"path"`
		FieldName        string   `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		Data dbFieldData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "DB_FIELD_STRING"},
		Data: dbFieldData{
			TriggerTableName: field.TriggerTableName,
			Path:             field.Path,
			FieldName:        field.FieldName,
		},
	})
}

func (field *DbFieldString) UnmarshalJSON(b []byte) error {
	// data schema
	var dbFieldData struct {
		StaticData struct {
			TriggerTableName string   `json:"triggerTableName"`
			Path             []string `json:"path"`
			FieldName        string   `json:"fieldName"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &dbFieldData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.TriggerTableName = dbFieldData.StaticData.TriggerTableName
	field.Path = dbFieldData.StaticData.Path
	field.FieldName = dbFieldData.StaticData.FieldName

	return nil
}

// ///////////////////////////////////////////////////////////////////////////////////////
// Payload field String
// ///////////////////////////////////////////////////////////////////////////////////////
type PayloadFieldString struct {
	FieldName string
}

// register creation
func init() {
	operatorFromType["PAYLOAD_FIELD_STRING"] = func() Operator { return &PayloadFieldString{} }
}

func (field PayloadFieldString) Eval(d DataAccessor) (string, error) {
	if !field.IsValid() {
		return "", ErrEvaluatingInvalidOperator
	}

	valRaw := d.GetPayloadField(field.FieldName)

	valPointer, ok := valRaw.(*string)
	if !ok {
		return "", fmt.Errorf("Payload field %s is not a pointer to a string", field.FieldName)
	}
	if valPointer == nil {
		return "", fmt.Errorf("Payload field %s is null", field.FieldName)
	}
	return *valPointer, nil
}

func (field PayloadFieldString) IsValid() bool {
	return field.FieldName != ""
}

func (field PayloadFieldString) String() string {
	return fmt.Sprintf("( String field from Payload: %s )", field.FieldName)
}

func (field PayloadFieldString) MarshalJSON() ([]byte, error) {

	// data schema
	type payloadFieldData struct {
		FieldName string `json:"fieldName"`
	}

	return json.Marshal(struct {
		OperatorType
		StaticData payloadFieldData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "PAYLOAD_FIELD_STRING"},
		StaticData: payloadFieldData{
			FieldName: field.FieldName,
		},
	})
}

func (field *PayloadFieldString) UnmarshalJSON(b []byte) error {
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
