package lockutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const parallel = 20

func TestWithDirLock(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")

	errc := make(chan error, 10)
	for i := 0; i < parallel; i++ {
		go func() {
			err := WithDirLock(dir, func() error {
				if _, err := os.Stat(log); err == nil {
					return nil
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
				logFile, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
				if err != nil {
					return err
				}
				defer logFile.Close()
				if _, err := fmt.Fprintf(logFile, "writer %d\n", i); err != nil {
					return err
				}
				return logFile.Close()
			})
			errc <- err
		}()
	}

	for i := 0; i < parallel; i++ {
		err := <-errc
		if err != nil {
			t.Error(err)
		}
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.Trim(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected one writer, got %v", lines)
	}
}
