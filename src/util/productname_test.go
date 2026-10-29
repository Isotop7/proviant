package util

import (
	"strings"
	"testing"
)

func TestProductNameLabel(t *testing.T) {
	t.Run("collapses the whitespace", func(t *testing.T) {
		if got := ProductNameLabel("  Milch   3.5%  "); got != "Milch 3.5%" {
			t.Errorf("ProductNameLabel() = %q, want %q", got, "Milch 3.5%")
		}
	})

	t.Run("keeps a short name", func(t *testing.T) {
		if got := ProductNameLabel("Milk"); got != "Milk" {
			t.Errorf("ProductNameLabel() = %q, want Milk", got)
		}
	})

	t.Run("caps a long name without splitting a rune", func(t *testing.T) {
		long := strings.Repeat("ä", MaxProductNameLength+10)
		got := ProductNameLabel(long)
		if len(got) > MaxProductNameLength {
			t.Errorf("length = %d, want <= %d", len(got), MaxProductNameLength)
		}
		if !strings.HasSuffix(got, "ä") {
			t.Errorf("the name was cut in the middle of a rune: %q", got)
		}
	})
}
