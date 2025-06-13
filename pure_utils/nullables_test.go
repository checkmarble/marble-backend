package pure_utils

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNullUUID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Null[uuid.UUID]
		wantErr bool
	}{
		{
			name:  "null value",
			input: "null",
			want: Null[uuid.UUID]{
				Valid: false,
				Set:   true,
			},
			wantErr: false,
		},
		{
			name:  "valid UUID",
			input: `"123e4567-e89b-12d3-a456-426614174000"`,
			want: Null[uuid.UUID]{
				value: uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Valid: true,
				Set:   true,
			},
			wantErr: false,
		},
		{
			name:    "invalid UUID format",
			input:   "\"not-a-uuid\"",
			want:    Null[uuid.UUID]{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   "invalid",
			want:    Null[uuid.UUID]{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Null[uuid.UUID]
			err := got.UnmarshalJSON([]byte(tt.input))

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestNullObject_UnmarshalJSON(t *testing.T) {
	type input struct {
		Int Null[int] `json:"int"`
	}

	tests := []struct {
		name    string
		input   string
		raw     int
		want    input
		wantErr bool
		wantNil bool
	}{
		{
			name:  "provided value",
			raw:   42,
			input: `{"int": 42}`,
			want:  input{Int: Null[int]{value: 42, Valid: true, Set: true}},
		},
		{
			name:    "omitted value",
			input:   `{}`,
			want:    input{Int: Null[int]{value: 0, Valid: false, Set: false}},
			wantNil: true,
		},
		{
			name:    "null value",
			input:   `{"int": null}`,
			want:    input{Int: Null[int]{value: 0, Valid: false, Set: true}},
			wantNil: true,
		},
		{
			name:    "invalid value",
			input:   `{"int": "hello"}`,
			want:    input{Int: Null[int]{value: 0, Valid: false, Set: true}},
			wantErr: true,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got input
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)

			if tt.wantNil {
				assert.Equal(t, 0, got.Int.Value())
				assert.Nil(t, got.Int.Ptr())
			} else {
				assert.Equal(t, tt.raw, got.Int.Value())
				assert.NotNil(t, got.Int.Ptr())
				assert.Equal(t, tt.raw, *got.Int.Ptr())
			}
		})
	}
}
