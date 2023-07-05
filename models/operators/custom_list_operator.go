package operators

import (
	"context"
	"encoding/json"
	"fmt"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// Db custom list String Array
// ///////////////////////////////////////////////////////////////////////////////////////
type DbCustomListStringArray struct {
	CustomListId string
} 

// register creation
func init() {
	operatorFromType["DB_CUSTOM_LIST_STRING_ARRAY"] = func() Operator { return &DbCustomListStringArray{} }
}

func (field DbCustomListStringArray) Eval(ctx context.Context, d DataAccessor) ([]string, error) {
	if !field.IsValid() {
		return nil, ErrEvaluatingInvalidOperator
	}

	return d.GetDbCustomListValues(ctx, field.CustomListId)
}

func (field DbCustomListStringArray) IsValid() bool {
	return field.CustomListId != ""
}

func (field DbCustomListStringArray) String() string {
	return fmt.Sprintf("( Custom list values from DB: custom list id %s )", field.CustomListId)
}

func (field DbCustomListStringArray) MarshalJSON() ([]byte, error) {

	// data schema
	type dbCustomListStringArrayData struct {
		CustomListId string   `json:"customListId"`
	}

	return json.Marshal(struct {
		OperatorType
		Data dbCustomListStringArrayData `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "DB_CUSTOM_LIST_STRING_ARRAY"},
		Data: dbCustomListStringArrayData{
			CustomListId: field.CustomListId,
		},
	})
}

func (field *DbCustomListStringArray) UnmarshalJSON(b []byte) error {
	// data schema
	var dbCustomListStringArrayData struct {
		StaticData struct {
			CustomListId string   `json:"customListId"`
		} `json:"staticData"`
	}

	if err := json.Unmarshal(b, &dbCustomListStringArrayData); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate staticData representation: %w", err)
	}
	field.CustomListId = dbCustomListStringArrayData.StaticData.CustomListId

	return nil
}
