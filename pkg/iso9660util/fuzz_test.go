package iso9660util

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func FuzzIsISO9660(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte) {
		imageFile := filepath.Join(t.TempDir(), "fuzz.iso")
		err := os.WriteFile(imageFile, fileContents, 0o600)
		assert.NilError(t, err)
		_, _ = IsISO9660(imageFile)
	})
}
