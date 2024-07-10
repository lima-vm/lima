package store

import (
	"os"
	"testing"
)

func FuzzLoadYAMLByFilePath(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte) {
		err := os.WriteFile("yaml_file.yml", fileContents, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove("yaml_file.yml")

		_, _ = LoadYAMLByFilePath("yaml_file.yml")
	})
}
