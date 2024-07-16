package store

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadYAMLByFilePath(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte) {
		localFile := filepath.Join(t.TempDir(), "yaml_file.yml")
		err := os.WriteFile(localFile, fileContents, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		//nolint:errcheck // The test doesn't check the return value
		LoadYAMLByFilePath(localFile)
	})
}
