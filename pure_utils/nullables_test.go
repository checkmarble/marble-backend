package pure_utils

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNullUUID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    NullUUID
		wantErr bool
	}{
		{
			name:  "null value",
			input: "null",
			want: NullUUID{
				Valid: false,
				Set:   true,
			},
			wantErr: false,
		},
		{
			name:  "valid UUID",
			input: "\"123e4567-e89b-12d3-a456-426614174000\"",
			want: NullUUID{
				UUID:  uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				Valid: true,
				Set:   true,
			},
			wantErr: false,
		},
		{
			name:    "invalid UUID format",
			input:   "\"not-a-uuid\"",
			want:    NullUUID{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   "invalid",
			want:    NullUUID{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got NullUUID
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
