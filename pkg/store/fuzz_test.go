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
		_, _ = LoadYAMLByFilePath(localFile)
	})
}
