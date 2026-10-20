package util

import "testing"

func TestClampProductQuantity(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"a count of zero becomes one pack", 0, 1},
		{"a negative count becomes one pack", -5, 1},
		{"a count inside the range is kept", 7, 7},
		{"a count above the maximum is capped", MaxProductQuantity + 1, MaxProductQuantity},
		{"the maximum itself is kept", MaxProductQuantity, MaxProductQuantity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClampProductQuantity(tt.got); got != tt.want {
				t.Errorf("ClampProductQuantity(%d) = %d, want %d", tt.got, got, tt.want)
			}
		})
	}
}
