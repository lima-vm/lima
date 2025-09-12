// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/usrlocalsharelima"
)

type Plugin struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

func DiscoverPlugins() ([]Plugin, error) {
	var plugins []Plugin
	seen := make(map[string]bool)

	dirs := getPluginDirectories()

	for _, dir := range dirs {
		pluginsInDir, err := scanDirectory(dir)
		if err != nil {
			logrus.Debugf("Failed to scan directory %s: %v", dir, err)
			continue
		}

		for _, plugin := range pluginsInDir {
			if !seen[plugin.Name] {
				plugins = append(plugins, plugin)
				seen[plugin.Name] = true
			}
		}
	}

	return plugins, nil
}

func getPluginDirectories() []string {
	var dirs []string

	selfPaths := []string{}

	selfViaArgs0, err := usrlocalsharelima.ExecutableViaArgs0()
	if err != nil {
		logrus.WithError(err).Debug("failed to find executable")
	} else {
		selfPaths = append(selfPaths, selfViaArgs0)
	}

	selfViaOS, err := os.Executable()
	if err != nil {
		logrus.WithError(err).Debug("failed to find os.Executable()")
	} else {
		selfFinalPathViaOS, err := filepath.EvalSymlinks(selfViaOS)
		if err != nil {
			logrus.WithError(err).Debug("failed to resolve symlinks")
			selfFinalPathViaOS = selfViaOS
		}

		if len(selfPaths) == 0 || selfFinalPathViaOS != selfPaths[0] {
			selfPaths = append(selfPaths, selfFinalPathViaOS)
		}
	}

	for _, self := range selfPaths {
		binDir := filepath.Dir(self)
		dirs = append(dirs, binDir)
	}

	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		pathDirs := filepath.SplitList(pathEnv)
		dirs = append(dirs, pathDirs...)
	}

	if prefixDir, err := usrlocalsharelima.Prefix(); err == nil {
		libexecDir := filepath.Join(prefixDir, "libexec", "lima")
		if _, err := os.Stat(libexecDir); err == nil {
			dirs = append(dirs, libexecDir)
		}
	}

	return dirs
}

func scanDirectory(dir string) ([]Plugin, error) {
	var plugins []Plugin

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "limactl-") {
			continue
		}

		pluginName := strings.TrimPrefix(name, "limactl-")

		if strings.Contains(pluginName, ".") {
			if filepath.Ext(name) == ".exe" {
				pluginName = strings.TrimSuffix(pluginName, ".exe")
			} else {
				continue
			}
		}

		fullPath := filepath.Join(dir, name)

		if !isExecutable(fullPath) {
			continue
		}

		plugin := Plugin{
			Name: pluginName,
			Path: fullPath,
		}

		if desc := extractDescFromScript(fullPath); desc != "" {
			plugin.Description = desc
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	mode := info.Mode()
	if mode&0o111 != 0 {
		return true
	}

	if filepath.Ext(path) == ".exe" {
		return true
	}

	return false
}

func extractDescFromScript(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		logrus.Debugf("Failed to read plugin script %s: %v", path, err)
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for pattern: # <limactl-desc>Description text</limactl-desc>
		if strings.HasPrefix(line, "#") && strings.Contains(line, "<limactl-desc>") && strings.Contains(line, "</limactl-desc>") {
			start := strings.Index(line, "<limactl-desc>") + len("<limactl-desc>")
			end := strings.Index(line, "</limactl-desc>")

			if start < end {
				desc := strings.TrimSpace(line[start:end])
				logrus.Debugf("Plugin %s: extracted description from script: %q", filepath.Base(path), desc)
				return desc
			}
		}
	}

	logrus.Debugf("Plugin %s: no <limactl-desc> found in script", filepath.Base(path))

	return ""
}

func UpdatePathForPlugins() error {
	pluginDirs := getPluginDirectories()
	newPath := strings.Join(pluginDirs, string(filepath.ListSeparator))
	return os.Setenv("PATH", newPath)
}
