// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"cmp"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/usrlocal"
)

const defaultPathExt = ".COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC;.CPL"

type Plugin struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

var Discover = sync.OnceValues(func() ([]Plugin, error) {
	var plugins []Plugin
	seen := make(map[string]bool)

	for _, dir := range getPluginDirectories() {
		for _, plugin := range scanDirectory(dir) {
			if !seen[plugin.Name] {
				plugins = append(plugins, plugin)
				seen[plugin.Name] = true
			}
		}
	}

	slices.SortFunc(plugins,
		func(i, j Plugin) int {
			return cmp.Compare(i.Name, j.Name)
		})

	return plugins, nil
})

var getPluginDirectories = sync.OnceValue(func() []string {
	dirs := usrlocal.SelfDirs()

	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		pathDirs := filepath.SplitList(pathEnv)
		dirs = append(dirs, pathDirs...)
	}

	libexecDirs, err := usrlocal.LibexecLima()
	if err == nil {
		dirs = append(dirs, libexecDirs...)
	}

	return dirs
})

// isWindowsExecutableExt checks if the given extension is a valid Windows executable extension
// according to PATHEXT environment variable.
func isWindowsExecutableExt(ext string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	pathExt := cmp.Or(os.Getenv("PATHEXT"), defaultPathExt)
	extensions := strings.Split(strings.ToUpper(pathExt), ";")
	return slices.Contains(extensions, strings.ToUpper(ext))
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !info.Mode().IsRegular() {
		return false
	}

	if runtime.GOOS != "windows" {
		return info.Mode()&0o111 != 0
	}

	return isWindowsExecutableExt(filepath.Ext(path))
}

func scanDirectory(dir string) []Plugin {
	var plugins []Plugin

	entries, err := os.ReadDir(dir)
	if err != nil {
		logrus.Debugf("Plugin discovery: failed to scan directory %s: %v", dir, err)

		return plugins
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

		if runtime.GOOS == "windows" {
			ext := filepath.Ext(pluginName)
			if isWindowsExecutableExt(ext) {
				pluginName = strings.TrimSuffix(pluginName, ext)
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

	return plugins
}

func (plugin *Plugin) Run(ctx context.Context, args []string) {
	if err := UpdatePath(); err != nil {
		logrus.Warnf("failed to update PATH environment: %v", err)
		// PATH update failure shouldn't prevent plugin execution
	}

	cmd := exec.CommandContext(ctx, plugin.Path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err := cmd.Run()
	osutil.HandleExitError(err)
	if err == nil {
		os.Exit(0) //nolint:revive // it's intentional to call os.Exit in this function
	}
	logrus.Fatalf("external command %q failed: %v", plugin.Path, err)
}

var descRegex = regexp.MustCompile(`<limactl-desc>(.*?)</limactl-desc>`)

func extractDescFromScript(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		logrus.Debugf("Failed to read plugin script %s: %v", path, err)
		return ""
	}

	if !strings.HasPrefix(string(content), "#!") {
		logrus.Debugf("Plugin %s: not a script file, skipping description extraction", path)
		return ""
	}

	matches := descRegex.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		logrus.Debugf("Plugin %s: no <limactl-desc> found in script", filepath.Base(path))
		return ""
	}

	desc := strings.Trim(matches[1], " ")
	logrus.Debugf("Plugin %s: extracted description: %q", filepath.Base(path), desc)
	return desc
}

// Find locates a plugin by name and returns a pointer to a copy.
func Find(name string) (*Plugin, error) {
	allPlugins, err := Discover()
	if err != nil {
		return nil, err
	}
	for _, plugin := range allPlugins {
		if name == plugin.Name {
			pluginCopy := plugin
			return &pluginCopy, nil
		}
	}
	return nil, nil
}

func UpdatePath() error {
	pluginDirs := getPluginDirectories()
	newPath := strings.Join(pluginDirs, string(filepath.ListSeparator))
	return os.Setenv("PATH", newPath)
}
