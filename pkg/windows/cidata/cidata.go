package cidata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type ISOConfig struct {
	XMLBytes    []byte
	ScriptBytes []byte
	OutputPath  string
}

func (c *ISOConfig) validate() error {
	switch {
	case len(c.XMLBytes) == 0:
		return fmt.Errorf("cidata: XMLBytes must not be empty")
	case len(c.ScriptBytes) == 0:
		return fmt.Errorf("cidata: ScriptBytes must not be empty")
	case c.OutputPath == "":
		return fmt.Errorf("cidata: OutputPath must not be empty")
	}
	return nil
}

func BuildISO(cfg ISOConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}

	tool, err := detectISOTool()
	if err != nil {
		return err
	}

	staging, err := os.MkdirTemp("", "lima-win-cidata-*")
	if err != nil {
		return fmt.Errorf("cidata: creating staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	files := map[string][]byte{
		"autounattend.xml": cfg.XMLBytes,
		"first_logon.ps1":  cfg.ScriptBytes,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(staging, name), data, 0o644); err != nil {
			return fmt.Errorf("cidata: writing %s: %w", name, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("cidata: creating output dir: %w", err)
	}

	tmp := cfg.OutputPath + ".tmp"
	if err := runISOTool(tool, staging, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	info, err := os.Stat(tmp)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("cidata: ISO tool produced an empty or missing file at %s", tmp)
	}

	if err := os.Rename(tmp, cfg.OutputPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cidata: finalising ISO: %w", err)
	}
	return nil
}

func detectISOTool() (string, error) {
	for _, candidate := range []string{"genisoimage", "mkisofs", "xorriso"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf(
		"cidata: no ISO tool found; install one of: genisoimage, mkisofs, xorriso\n" +
			"  Ubuntu/Debian: sudo apt install genisoimage\n" +
			"  Fedora/RHEL:   sudo dnf install genisoimage",
	)
}

func runISOTool(toolPath, stagingDir, outputPath string) error {
	cmd := exec.Command(toolPath,
		"-output", outputPath,
		"-joliet",
		"-rock",
		"-volid", "cidata",
		stagingDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cidata: %s failed: %w\noutput: %s", filepath.Base(toolPath), err, out)
	}
	return nil
}
