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
	assert.Equal(t, result, []string{"1", "2"})
}

func TestMap_Nil(t *testing.T) {
	assert.Nilf(t, Map(nil, dummy), "should return nil when src is nil")
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
	assert.Equal(t, result, []string{"1", "2", ""})
	assert.ErrorIs(t, err, errorForTesting)
}

func TestMapErr_Nil(t *testing.T) {
	result, err := MapErr(nil, dummyErr)
	assert.NoError(t, err)
	assert.Nil(t, result, "should return nil when src is nil")
}

func TestMapMap(t *testing.T) {

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result := MapMap(values, func(v int) string {
		return fmt.Sprintf("%d", v)
	})
	assert.Equal(t, result, map[string]string{"a": "1", "b": "2", "c": "3"})
}

func TestMapMap_Nil(t *testing.T) {
	assert.Nilf(t, MapMap[int](nil, dummy), "should return nil when src is nil")
}

func TestMapMapErr(t *testing.T) {

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result, err := MapMapErr(values, func(v int) (string, error) {
		return fmt.Sprintf("%d", v), nil
	})
	assert.NoError(t, err)
	assert.Equal(t, result, map[string]string{"a": "1", "b": "2", "c": "3"})
}

func TestMapMapErr_WithError(t *testing.T) {
	errorForTesting := errors.New("testing error")

	values := map[string]int{"a": 1, "b": 2, "c": 3}
	result, err := MapMapErr(values, func(v int) (string, error) {
		if v == 1 {
			return "1", nil
		}
		return "2", errorForTesting
	})
	assert.Nil(t, result)
	assert.ErrorIs(t, err, errorForTesting)
}

func TestMapMapErr_Nil(t *testing.T) {
	result, err := MapMapErr[int](nil, dummyErr)
	assert.NoError(t, err)
	assert.Nilf(t, result, "should return nil when src is nil")
}
