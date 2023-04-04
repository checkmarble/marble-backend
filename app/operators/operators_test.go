package operators

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

type DataAccessorImpl struct{}

func (d *DataAccessorImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}
func (d *DataAccessorImpl) GetDBField(path []string, fieldName string) (interface{}, error) {
	return nil, nil
}

func TestLogic(t *testing.T) {
	tree := EqBool{
		Left: &True{},
		Right: &EqBool{
			Left:  &False{},
			Right: &False{},
		},
	}
	dataAccessor := DataAccessorImpl{}

	expected := true
	got := tree.Eval(&dataAccessor)

	if got != expected {
		t.Errorf("got: %v, expected: %v", got, expected)
	}
}

func TestLogic2(t *testing.T) {
	tree := EqBool{
		Left: &True{},
		Right: &EqBool{
			Left:  &False{},
			Right: &True{},
		},
	}
	dataAccessor := DataAccessorImpl{}

	expected := false
	got := tree.Eval(&dataAccessor)

	if got != expected {
		t.Errorf("got: %v, expected: %v", got, expected)
	}
}

func TestMarshalUnMarshal(t *testing.T) {
	tree := EqBool{
		Left: &True{},
		Right: &EqBool{
			Left:  &False{},
			Right: &True{},
		},
	}
	dataAccessor := DataAccessorImpl{}

	JSONbytes, err := tree.MarshalJSON()
	if err != nil {
		t.Errorf("error marshaling operator: %v", err)
	}

	t.Log(string(JSONbytes))

	rootOperator, err := unmarshalOperatorBool(JSONbytes)
	if err != nil {
		t.Errorf("error unmarshaling operator: %v", err)
	}

	spew.Dump(tree)
	spew.Dump(rootOperator)

	expected := tree.Eval(&dataAccessor)
	got := rootOperator.Eval(&dataAccessor)

	if got != expected {
		t.Errorf("got: %v, expected: %v", got, expected)
	}
}

func TestMarshalContracts(t *testing.T) {
	for typeKey, creatorFunc := range operatorFromType {

		testname := typeKey
		t.Run(testname, func(t *testing.T) {

			op := creatorFunc()
			JSONop, err := op.MarshalJSON()
			if err != nil {
				t.Errorf("unable to marshal operator to JSON")
			}

			if !bytes.Contains(JSONop, []byte("data")) {
				t.Errorf("marshaled operator does not contain `data`")
			}
			if !bytes.Contains(JSONop, []byte(fmt.Sprintf("\"type\":\"%s\"", typeKey))) {
				t.Errorf("marshaled operator does not contain `\"type\":\"%s\"`", typeKey)
			}

		})
	}

}
