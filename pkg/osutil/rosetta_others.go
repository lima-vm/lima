//go:build !darwin

package osutil

func IsBeingRosettaTranslated() bool {
	return false
}
