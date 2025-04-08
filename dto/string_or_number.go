package dto

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
)

type StringOrNumber struct {
	StringValue *string
	FloatValue  *float64
}

func (c *StringOrNumber) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.StringValue = &s
		return nil
	}

	// Try number
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		c.FloatValue = &n
		return nil
	}

	return fmt.Errorf("c must be either a string or a number")
}

func (c StringOrNumber) MarshalJSON() ([]byte, error) {
	if c.StringValue != nil {
		return json.Marshal(c.StringValue)
	}
	if c.FloatValue != nil {
		return json.Marshal(c.FloatValue)
	}
	return json.Marshal(nil)
}

func AdaptStringOrNumber(input StringOrNumber) models.StringOrNumber {
	return models.StringOrNumber{
		StringValue: input.StringValue,
		FloatValue:  input.FloatValue,
	}
}

func AdaptStringOrNumberDto(input models.StringOrNumber) StringOrNumber {
	return StringOrNumber{
		StringValue: input.StringValue,
		FloatValue:  input.FloatValue,
	}
}
