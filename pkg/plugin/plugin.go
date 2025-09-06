// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
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

	if exe, err := os.Executable(); err == nil {
		binDir := filepath.Dir(exe)
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

		if desc := getPluginDescription(fullPath); desc != "" {
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

func getPluginDescription(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	descRegex := regexp.MustCompile(`<limactl>\s*([^<]+?)\s*</limactl>`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		matches := descRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

func UpdatePathForPlugins() error {
	pluginDirs := getPluginDirectories()
	newPath := strings.Join(pluginDirs, string(filepath.ListSeparator))
	return os.Setenv("PATH", newPath)
}
