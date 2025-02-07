package pure_utils

import "testing"

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"same elements, different order", []string{"a", "b", "c"}, []string{"c", "b", "a"}, true},
		{"same elements, with repetitions", []string{"a", "b", "c"}, []string{"a", "b", "b", "c"}, true},
		{"different lengths", []string{"a", "b"}, []string{"a", "b", "c"}, false},
		{"different elements", []string{"a", "b", "c"}, []string{"a", "b", "d"}, false},
		{"empty slices", []string{}, []string{}, true},
		{"one empty slice", []string{"a", "b", "c"}, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsSameElements(tt.a, tt.b); got != tt.want {
				t.Errorf("SlicesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
