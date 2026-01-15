package repositories

import (
	"reflect"
	"testing"

	"github.com/checkmarble/marble-backend/api-clients/convoy"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetOwnerId(t *testing.T) {
	type args struct {
		organizationId uuid.UUID
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "organization only",
			args: args{organizationId: uuid.MustParse("12456789-1234-5678-9012-345678901234")},
			want: "org:12456789-1234-5678-9012-345678901234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getOwnerId(tt.args.organizationId)
			if !reflect.DeepEqual(got, tt.want) {
				partnerId := "nil"

				t.Errorf("getOwnerId(%s, %s) got = %v, want %v",
					tt.args.organizationId, partnerId, got, tt.want)
			}
		})
	}
}

func TestParseOwnerId(t *testing.T) {
	type want struct {
		organizationId uuid.UUID
	}
	tests := []struct {
		name string
		args string
		want want
	}{
		{
			name: "organization only",
			args: "org:12456789-1234-5678-9012-345678901234",
			want: want{organizationId: uuid.MustParse("12456789-1234-5678-9012-345678901234")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			organizationId := parseOwnerId(tt.args)
			assert.Equal(t, tt.want.organizationId, organizationId)
		})
	}
}

func TestGetName(t *testing.T) {
	type args struct {
		ownerId    string
		eventTypes []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "all events",
			args: args{ownerId: "org:12456"},
			want: "org:12456|all-events",
		},
		{
			name: "with a single event",
			args: args{ownerId: "org:12456", eventTypes: []string{"event1"}},
			want: "org:12456|event1",
		},
		{
			name: "with a multiple events",
			args: args{ownerId: "org:12456", eventTypes: []string{"event1", "event2"}},
			want: "org:12456|event1,event2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getName(tt.args.ownerId, tt.args.eventTypes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAdaptEvenType(t *testing.T) {
	tests := []struct {
		name string
		args convoy.DatastoreFilterConfiguration
		want []string
	}{
		{
			name: "nil event types",
			args: convoy.DatastoreFilterConfiguration{EventTypes: nil},
			want: nil,
		},
		{
			name: "with event types",
			args: convoy.DatastoreFilterConfiguration{EventTypes: utils.Ptr([]string{"event1"})},
			want: []string{"event1"},
		},
		{
			name: "with * event types",
			args: convoy.DatastoreFilterConfiguration{EventTypes: utils.Ptr([]string{"*"})},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adaptEventTypes(tt.args)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestAdaptModelsFilterConfiguration(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want convoy.ModelsFilterConfiguration
	}{
		{
			name: "with event types",
			args: []string{"event1"},
			want: convoy.ModelsFilterConfiguration{EventTypes: utils.Ptr([]string{"event1"})},
		},
		{
			name: "empty event types",
			args: []string{},
			want: convoy.ModelsFilterConfiguration{EventTypes: utils.Ptr([]string{"*"})},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adaptModelsFilterConfiguration(tt.args)
			assert.Equal(t, got, tt.want)
		})
	}
}
