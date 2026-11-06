package models

import "testing"

func TestScreeningConfigHasSameQueryThreshold(t *testing.T) {
	tests := []struct {
		name  string
		left  *int
		right *int
		want  bool
	}{
		{name: "equal values in distinct pointers", left: new(70), right: new(70), want: true},
		{name: "different values", left: new(70), right: new(80), want: false},
		{name: "both inherit the organization threshold", want: true},
		{name: "only left overrides the threshold", left: new(70), want: false},
		{name: "only right overrides the threshold", right: new(70), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := ScreeningConfig{Threshold: tt.left}
			right := ScreeningConfig{Threshold: tt.right}
			if got := left.HasSameQuery(right); got != tt.want {
				t.Errorf("HasSameQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
