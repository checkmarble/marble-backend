package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var dummy = func(int) int { return 0 }
var dummyErr = func(int) (int, error) { return 0, nil }

func TestMap(t *testing.T) {
	values := []int{1, 2}
	result := Map(values, func(v int) string {
		return fmt.Sprintf("%d", v)
	})
	assert.Equal(t, []string{"1", "2"}, result)
}

func TestMap_Nil(t *testing.T) {
	assert.Empty(t, Map(nil, dummy), "should return empty slice when src is nil")
}

func TestMapErr(t *testing.T) {

	errorForTesting := errors.New("testing error")

	values := []int{1, 2, 3}
	result, err := MapErr(values, func(v int) (string, error) {
		if v == 1 {
			return "1", nil
		}
		return "2", errorForTesting
	})
	assert.Empty(t, result)
	assert.ErrorIs(t, err, errorForTesting)
}

func TestMapErr_Nil(t *testing.T) {
	result, err := MapErr(nil, dummyErr)
	assert.NoError(t, err)
	assert.Empty(t, result, "should return empty slice when src is nil")
}

func TestMapValues(t *testing.T) {

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result := MapValues(values, func(v int) string {
		return fmt.Sprintf("%d", v)
	})
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, result)
}

func TestMapValues_Nil(t *testing.T) {
	assert.Empty(t, MapValues[int](nil, dummy), "should return empty map when src is nil")
}

func TestMapValuesErr(t *testing.T) {

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result, err := MapValuesErr(values, func(v int) (string, error) {
		return fmt.Sprintf("%d", v), nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, result)
}

func TestMapValuesErr_WithError(t *testing.T) {
	errorForTesting := errors.New("testing error")

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result, err := MapValuesErr(values, func(v int) (string, error) {
		if v == 1 {
			return "1", nil
		}
		return "2", errorForTesting
	})
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errorForTesting)
}

func TestMapValuesErr_Nil(t *testing.T) {
	result, err := MapValuesErr[int](nil, dummyErr)
	assert.NoError(t, err)
	assert.Empty(t, result, "should return empty map when src is nil")
}
