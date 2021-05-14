package iso9660util

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/pkg/errors"
)

func Write(isoFilePath string, iso *ISO9660) error {
	td, err := ioutil.TempDir("", "lima-iso9660util")
	if err != nil {
		return err
	}
	defer os.RemoveAll(td)
	for k, v := range iso.FilesFromContent {
		if strings.ToLower(k) != k {
			return errors.Errorf("file name must be lower characters, got %q", k)
		}
		f, err := securejoin.SecureJoin(td, k)
		if err != nil {
			return err
		}
		if err := os.WriteFile(f, []byte(v), 0644); err != nil {
			return err
		}
	}

	for k, v := range iso.FilesFromHostFilePath {
		if strings.ToLower(k) != k {
			return errors.Errorf("file name must be lower characters, got %q", k)
		}
		f, err := securejoin.SecureJoin(td, k)
		if err != nil {
			return err
		}
		cmd := exec.Command("cp", "-a", v, f)
		if out, err := cmd.CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to run %v: %q", cmd.Args, string(out))
		}
	}

	if err := os.RemoveAll(isoFilePath); err != nil {
		return err
	}

	cmd := exec.Command("hdiutil", "makehybrid", "-iso", "-joliet",
		"-o", isoFilePath,
		"-default-volume-name", iso.Name,
		td)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to run %v: %q", cmd.Args, string(out))
	}
	return nil
}
