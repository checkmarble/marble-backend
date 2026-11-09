package models

import (
	"testing"

	"github.com/sosodev/duration"
	"github.com/stretchr/testify/assert"
)

func TestLifecycleIndexes(t *testing.T) {
	d := &duration.Duration{Days: 31}
	active := ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}}
	stale := ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleStale, Indexed: []string{"valid_until"}}
	tests := []struct {
		name   string
		policy TableLifecycle
		want   []ConcreteIndex
	}{
		{"disabled", TableLifecycle{DeleteActiveRowsAfter: d, DeleteStaleRowsAfter: d}, nil},
		{"none", TableLifecycle{Enabled: true}, nil},
		{"active", TableLifecycle{Enabled: true, DeleteActiveRowsAfter: d}, []ConcreteIndex{active}},
		{"stale", TableLifecycle{Enabled: true, DeleteStaleRowsAfter: d}, []ConcreteIndex{stale}},
		{"both", TableLifecycle{Enabled: true, DeleteActiveRowsAfter: d, DeleteStaleRowsAfter: d}, []ConcreteIndex{active, stale}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LifecycleIndexes("transactions", tt.policy))
		})
	}
}

func TestLifecycleIndexExists(t *testing.T) {
	required := ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}}
	tests := []struct {
		name  string
		index ConcreteIndex
		want  bool
	}{
		{"valid", ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}, Status: IndexStatusValid}, true},
		{"pending", ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}, Status: IndexStatusPending}, false},
		{"invalid", ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}, Status: IndexStatusInvalid}, false},
		{"other table", ConcreteIndex{TableName: "accounts", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_from"}, Status: IndexStatusValid}, false},
		{"other type", ConcreteIndex{TableName: "transactions", Type: IndexTypeNavigation, Indexed: []string{"valid_from"}, Status: IndexStatusValid}, false},
		{"other column", ConcreteIndex{TableName: "transactions", Type: IndexTypeLifecycleActive, Indexed: []string{"valid_until"}, Status: IndexStatusValid}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, LifecycleIndexExists(required, []ConcreteIndex{tt.index}))
		})
	}
}
