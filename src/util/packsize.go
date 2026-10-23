package util

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// countUnits are the unit symbols that always counted items instead of
// measuring them. A product stored in one of them held a count; anything else
// held a net content in a base unit. The list carries the spellings the old
// conversion table accepted, so a legacy row with a plural or an alias
// ("jars", "pc", "packages") is still read as a count. It also carries the
// free-text units the old demo data used ("bags", "loaf"), which the
// conversion table never knew and therefore never listed.
var countUnits = []string{
	"pcs", "pc", "piece", "pieces",
	"pkg", "pack", "packs", "package", "packages",
	"can", "cans",
	"box", "boxes",
	"jar", "jars",
	"bottle", "bottles",
	"bunch", "bunches",
	"slice", "slices",
	"bag", "bags",
	"loaf", "loaves",
}

// IsCountUnit reports whether a unit symbol counts items instead of measuring
// them. An empty or unrecognised symbol is not a count: it is a unit this
// build cannot read, and treating it as one would silently drop a net
// content.
func IsCountUnit(raw string) bool {
	symbol := strings.ToLower(strings.TrimSpace(raw))
	for _, unit := range countUnits {
		if symbol == unit {
			return true
		}
	}
	return false
}

// MaxPackSizeLength bounds a pack size label. The source is a free-text field,
// so the text is collapsed and capped before it is stored.
const MaxPackSizeLength = 60

// PackSizeLabel normalises a pack size for display: the whitespace is
// collapsed and the text is capped, because the source is a free-text field
// that a third party fills in.
func PackSizeLabel(raw string) string {
	collapsed := strings.Join(strings.Fields(raw), " ")
	if len(collapsed) > MaxPackSizeLength {
		collapsed = truncatedUTF8(collapsed, MaxPackSizeLength)
	}
	return collapsed
}

// truncatedUTF8 cuts a string to a byte limit without splitting a rune.
func truncatedUTF8(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return strings.TrimSpace(value[:cut])
}

// PackSizeFromLegacyAmount converts a stored net content into a pack count
// and a pack size. A measured amount described the content of one pack, so it
// becomes a single pack whose size is that amount; a counted amount was
// already a count. The pack size is the amount and its unit as text, because
// the pack size is display metadata and is never converted.
func PackSizeFromLegacyAmount(amount int, unit string) (quantity int, packSize string) {
	trimmed := strings.TrimSpace(unit)
	if amount < 1 {
		return 1, ""
	}
	if trimmed == "" || IsCountUnit(trimmed) {
		return amount, ""
	}
	return 1, fmt.Sprintf("%d %s", amount, trimmed)
}
