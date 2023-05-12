package operators

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMarshalContracts(t *testing.T) {
	for typeKey, creatorFunc := range operatorFromType {
		testname := typeKey
		t.Run(testname, func(t *testing.T) {

			op := creatorFunc()
			JSONop, err := op.MarshalJSON()
			if err != nil {
				t.Errorf("unable to marshal operator to JSON")
			}

			var mapFormatOp map[string]interface{}
			err = json.Unmarshal(JSONop, &mapFormatOp)
			fmt.Println(mapFormatOp)
			for k := range mapFormatOp {
				if k != "type" && k != "staticData" && k != "children" {
					t.Errorf("marshaled operator contains unexpected key: %s", k)
				}
			}
			_, ok := mapFormatOp["type"]
			if !ok {
				t.Errorf(`marshaled operator does not contain mandatory field "type"`)
			}
		})
	}
}
