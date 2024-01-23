//go:build !darwin

package osutil

func SetExcludeFromBackup(path string, exclude bool) {}
