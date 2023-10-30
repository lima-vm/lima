// Package ptr holds utilities for taking pointer references to values.
package ptr

// Of returns pointer to value.
func Of[T any](value T) *T {
	return &value
}
