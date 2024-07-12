package iso9660util

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzIsISO9660(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte) {
		imageFile := filepath.Join(t.TempDir(), "fuzz.iso")
		err := os.WriteFile(imageFile, fileContents, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		//nolint:errcheck // The test doesn't check the return value
		IsISO9660(imageFile)
	})
}
