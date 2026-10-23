package util

import "testing"

// A legacy row carries the unit the old conversion table accepted. The plural
// and alias spellings count items, not net contents.
func TestPackSizeFromLegacyAmount(t *testing.T) {
	counted := []string{"pcs", "pc", "piece", "pieces", "pkg", "pack", "packages", "jars", "cans", "boxes", "bottles", "bags", "loaf"}
	for _, unit := range counted {
		quantity, packSize := PackSizeFromLegacyAmount(3, unit)
		if quantity != 3 || packSize != "" {
			t.Errorf("PackSizeFromLegacyAmount(3, %q) = %d, %q, want 3 and no pack size", unit, quantity, packSize)
		}
	}

	measured := []string{"g", "kg", "ml", "l"}
	for _, unit := range measured {
		quantity, packSize := PackSizeFromLegacyAmount(500, unit)
		if quantity != 1 || packSize != "500 "+unit {
			t.Errorf("PackSizeFromLegacyAmount(500, %q) = %d, %q, want 1 and %q", unit, quantity, packSize, "500 "+unit)
		}
	}

	quantity, packSize := PackSizeFromLegacyAmount(0, "g")
	if quantity != 1 || packSize != "" {
		t.Errorf("PackSizeFromLegacyAmount(0, \"g\") = %d, %q, want 1 and no pack size", quantity, packSize)
	}
}
