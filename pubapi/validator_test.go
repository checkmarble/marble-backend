package pubapi

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

//nolint:tagliatelle
type validatee struct {
	Required    int    `json:"required" validate:"required"`
	OneOf       string `json:"oneof" validate:"oneof=one two three"`
	Gt          int    `json:"gt" validate:"gt=2"`
	Lt          int    `json:"lt" validate:"lt=2"`
	GtField     int    `json:"gtfield" validate:"gtcsfield=Gt"`
	LtField     int    `json:"ltfield" validate:"ltcsfield=Gt"`
	Len         []int  `json:"len" validate:"len=3"`
	Bool        string `json:"bool" validate:"boolean"`
	Date        string `json:"date" validate:"datetime=2006-01-02T15:04:05-07:00"`
	Untagged    string `validate:"required"`
	UnnamedJson string `json:"," validate:"required"`
	SkippedJson string `json:"-" validate:"required"`
	FormField   int    `form:"form_field" validate:"required"`
	OtherCheck  string `json:"other" validate:"cidrv4"`
}

func testValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(fieldNameFromTag)

	return v
}

func TestPublicApiValidatorMessagesOk(t *testing.T) {
	v := validator.New()
	o := validatee{
		Required:    1,
		OneOf:       "one",
		Gt:          3,
		Lt:          1,
		GtField:     4,
		LtField:     0,
		Len:         []int{1, 2, 3},
		Bool:        "true",
		Date:        "2024-04-17T18:29:00+02:00",
		Untagged:    "OK",
		UnnamedJson: "OK",
		SkippedJson: "OK",
		FormField:   1,
		OtherCheck:  "10.10.0.0/16",
	}

	assert.NoError(t, v.Struct(o))
}

func TestPublicApiPropreValidationMessages(t *testing.T) {
	v := testValidator()
	o := validatee{
		OneOf:      "four",
		Gt:         1,
		Lt:         3,
		GtField:    0,
		LtField:    2,
		Len:        []int{0},
		Bool:       "nope",
		Date:       "nope",
		OtherCheck: "not_a_cidr",
	}

	err := v.Struct(o)

	assert.Error(t, err)

	verr := err.(validator.ValidationErrors) // nolint:errorlint
	errs := make([]string, len(verr))

	for idx, e := range verr {
		errs[idx] = AdaptFieldValidationError(e)
	}

	assert.Contains(t, errs, "field `required` is required")
	assert.Contains(t, errs, "field `oneof` must be one of one, two, three")
	assert.Contains(t, errs, "field `gt` must be greater than 2")
	assert.Contains(t, errs, "field `lt` must be less than 2")
	assert.Contains(t, errs, "field `gtfield` must be greater than the value of `Gt`")
	assert.Contains(t, errs, "field `ltfield` must be less than the value of `Gt`")
	assert.Contains(t, errs, "field `len` must be of length 3")
	assert.Contains(t, errs, "field `bool` should be 'true' or 'false'")
	assert.Contains(t, errs, "field `date` should be in the format '2006-01-02T15:04:05Z07:00'")
	assert.Contains(t, errs, "field `Untagged` is required")
	assert.Contains(t, errs, "field `UnnamedJson` is required")
	assert.Contains(t, errs, "field `SkippedJson` is required")
	assert.Contains(t, errs, "field `form_field` is required")
	assert.Contains(t, errs, "field `other` is invalid")
}
