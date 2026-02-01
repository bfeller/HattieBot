package tools

import (
	"strconv"
)

// suffixReserve is runes reserved for the truncation message (approximate; actual suffix length varies with digit count).
const suffixReserve = 80

// TruncateToolOutput caps s at maxRunes runes. If maxRunes <= 0, returns s unchanged.
// Truncation preserves the start of the string and appends a suffix with total rune count.
// Truncated JSON may be invalid; the model can retry with a smaller scope (e.g. read a portion of a file).
func TruncateToolOutput(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	keep := maxRunes - suffixReserve
	if keep <= 0 {
		keep = 1
	}
	suffix := "\n...[output truncated, total " + strconv.Itoa(len(r)) + " runes]"
	return string(r[:keep]) + suffix
}
