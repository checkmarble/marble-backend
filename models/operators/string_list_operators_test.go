package operators

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

type DataAccessorStringListImpl struct{}

func (d *DataAccessorStringListImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return nil, nil
}

func (d *DataAccessorStringListImpl) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return nil, nil
}

func (d *DataAccessorStringListImpl) GetDbHandle() (db *pgxpool.Pool, schema string, err error) {
	return nil, "", nil
}

func (d *DataAccessorStringListImpl) GetDbCustomListValues(ctx context.Context, customListId string) ([]string, error) {
	if customListId == "test-test-test-test" {
		return []string{"test", "test2"}, nil
	}
	return nil, nil
}

func TestLogicEvalStringList(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorStringList
		expected []string
	}
	dataAccessor := DataAccessorStringListImpl{}

	cases := []testCase{
		{
			name: "scalar",
			operator: &StringListValue{
				Value: []string{"abc", "def"},
			},
			expected: []string{"abc", "def"},
		},
		{
			name: "db custom list",
			operator: &DbCustomListStringArray{
				CustomListId: "test-test-test-test",
			},
			expected: []string{"test", "test2"},
		},
	}
	asserts := assert.New(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.operator.Eval(context.Background(), &dataAccessor)

			if err != nil {
				t.Errorf("error: %v", err)
			}

			asserts.Equal(c.expected, got)
		})
	}
}

func TestMarshalUnMarshalStringList(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorStringList
	}
	dataAccessor := DataAccessorStringImpl{}
	asserts := assert.New(t)

	cases := []testCase{
		{
			name:     "Constant value",
			operator: &StringListValue{Value: []string{"abc", "def"}},
		},
		{
			name:     "Constant value",
			operator: &DbCustomListStringArray{CustomListId: "test-test-test-test"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JSONbytes, err := c.operator.MarshalJSON()
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}

			t.Log("JSONbytes", string(JSONbytes))

			rootOperator, err := UnmarshalOperatorStringList(JSONbytes)
			if err != nil {
				t.Errorf("error unmarshaling operator: %v", err)
			}
			fmt.Println(rootOperator)

			expected, err := c.operator.Eval(context.Background(), &dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}
			got, err := rootOperator.Eval(context.Background(), &dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}

			asserts.Equal(expected, got, "evaluated operator should be the same as the original")

		})
	}
}
