package nativeimgutil

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzConvertToRaw(f *testing.F) {
	f.Fuzz(func(t *testing.T, imgData []byte, withBacking bool, size int64) {
		srcPath := filepath.Join(t.TempDir(), "src.img")
		destPath := filepath.Join(t.TempDir(), "dest.img")
		err := os.WriteFile(srcPath, imgData, 0o600)
		if err != nil {
			return
		}
		_ = ConvertToRaw(srcPath, destPath, &size, withBacking)
	})
}
