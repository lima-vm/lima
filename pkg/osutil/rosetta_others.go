//go:build !darwin
// +build !darwin

package osutil

func IsBeingRosettaTranslated() bool {
	return false
}
