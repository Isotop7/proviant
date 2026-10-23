package v1

import "testing"

// TestValidEAN13Checksum pins the EAN-13 check digit: the first twelve digits
// carry the weights one and three, alternating from the first digit, and the
// check digit itself carries the weight one. The codes are real products, so
// a wrong weighting rejects them.
func TestValidEAN13Checksum(t *testing.T) {
	valid := []string{
		"3073781116090", // Milbona
		"4001724814405", // Nutella
		"4006381333931", // Milka
		"3387390325911", // Oatly
		"5053827182225", // Alpro
		"4099100017885", // Milram
	}
	for _, barcode := range valid {
		if !validEAN13Checksum(barcode) {
			t.Errorf("validEAN13Checksum(%q) = false, want true", barcode)
		}
	}

	// A changed check digit never keeps the weighted sum a multiple of ten.
	invalid := []string{
		"3073781116091",
		"4001724814400",
		"4006381333930",
		"3387390325912",
	}
	for _, barcode := range invalid {
		if validEAN13Checksum(barcode) {
			t.Errorf("validEAN13Checksum(%q) = true, want false", barcode)
		}
	}
}
