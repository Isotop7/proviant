package util

import "strings"

// MaxProductNameLength bounds a product name. The column is a bounded
// varchar, so a longer name cannot be stored: a client that sends one is cut
// instead of failing the whole request.
const MaxProductNameLength = 191

// ProductNameLabel normalises a product name for storage: the whitespace is
// collapsed and the text is capped, because the source is a free-text field
// that a client fills in.
func ProductNameLabel(raw string) string {
	collapsed := strings.Join(strings.Fields(raw), " ")
	if len(collapsed) > MaxProductNameLength {
		collapsed = truncatedUTF8(collapsed, MaxProductNameLength)
	}
	return collapsed
}
