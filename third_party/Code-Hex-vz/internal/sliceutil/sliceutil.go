package sliceutil

// FindValueByIndex returns the value of the index in s,
// or zero value if not present.
func FindValueByIndex[S ~[]E, E any](s S, idx int) (v E) {
	if idx < 0 || idx >= len(s) {
		return v // return zero value of type E
	}
	return s[idx]
}
