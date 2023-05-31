package operators

import (
	"context"
	"encoding/json"
	"fmt"
)

// ///////////////////////////////////////////////////////////////////////////////////////
// Variable: generic operator taking a string as input (child) and returning a float as output, for
// a given variable (read from repository)
// ///////////////////////////////////////////////////////////////////////////////////////

type VariableStringInputFloatOutput struct {
	VariableId string
	Operand    OperatorString
}

// register creation
func init() {
	operatorFromType["VARIABLE_STRING_PARAM_FLOAT_OUTPUT"] = func() Operator { return &VariableStringInputFloatOutput{} }
}

func (r VariableStringInputFloatOutput) Eval(ctx context.Context, d DataAccessor) (float64, error) {
	variable, err := d.GetVariable(ctx, r.VariableId)
	if err != nil {
		return 0, err
	}
	db := d.GetDbHandle()

	variableParam, err := r.Operand.Eval(ctx, d)
	if err != nil {
		return 0, err
	}

	// Execute query with the account id
	sql := variable.SqlTemplate
	args := []any{variableParam}
	rows := db.QueryRow(ctx, sql, args...)

	var output float64
	err = rows.Scan(&output)
	if err != nil {
		return 0, err
	}

	return output, nil
}

func (r VariableStringInputFloatOutput) IsValid() bool {
	return r.VariableId != "" && r.Operand != nil && r.Operand.IsValid()
}

func (r VariableStringInputFloatOutput) String() string {
	return fmt.Sprintf("[Complex variable %s with input %s]", r.VariableId, r.Operand.String())
}

func (r VariableStringInputFloatOutput) MarshalJSON() ([]byte, error) {
	type data struct {
		VariableId string `json:"variable_id"`
	}

	return json.Marshal(struct {
		OperatorType
		Children   []OperatorString `json:"children"`
		StaticData data             `json:"staticData"`
	}{
		OperatorType: OperatorType{Type: "VARIABLE_STRING_PARAM_FLOAT_OUTPUT"},
		Children:     []OperatorString{r.Operand},
		StaticData:   data{VariableId: r.VariableId},
	})
}

func (r *VariableStringInputFloatOutput) UnmarshalJSON(b []byte) error {
	// data schema
	var data struct {
		Children   []json.RawMessage `json:"children"`
		StaticData struct {
			VariableId string `json:"variable_id"`
		} `json:"staticData"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("unable to unmarshal operator to intermediate children representation: %w", err)
	}

	// Build concrete child Operand
	child, err := UnmarshalOperatorString(data.Children[0])
	if err != nil {
		return fmt.Errorf("unable to instantiate child operator: %w", err)
	}
	r.Operand = child
	r.VariableId = data.StaticData.VariableId

	return nil
}
