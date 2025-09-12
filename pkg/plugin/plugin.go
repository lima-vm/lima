// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugin

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
		pluginsInDir := scanDirectory(dir)
		for _, plugin := range pluginsInDir {
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
}

func getPluginDirectories() []string {
	var dirs []string

	selfDirs := usrlocalsharelima.SelfDirs()
	dirs = append(dirs, selfDirs...)

	pathEnv := os.Getenv("PATH")
	if pathEnv != "" {
		pathDirs := filepath.SplitList(pathEnv)
		dirs = append(dirs, pathDirs...)
	}

	if libexecDir, err := usrlocalsharelima.LibexecLima(); err == nil {
		if _, err := os.Stat(libexecDir); err == nil {
			dirs = append(dirs, libexecDir)
		}
	}

	return dirs
}

// isWindowsExecutableExt checks if the given extension is a valid Windows executable extension
// according to PATHEXT environment variable.
func isWindowsExecutableExt(ext string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	pathExt := os.Getenv("PATHEXT")
	if pathExt == "" {
		pathExt = ".COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC;.CPL"
	}

	extensions := strings.Split(strings.ToUpper(pathExt), ";")
	extUpper := strings.ToUpper(ext)

	for _, validExt := range extensions {
		if validExt == extUpper {
			return true
		}
	}
	return false
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

		if strings.Contains(pluginName, ".") {
			ext := filepath.Ext(name)
			if !isWindowsExecutableExt(ext) {
				continue
			}

			pluginName = strings.TrimSuffix(pluginName, ext)
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

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	mode := info.Mode()
	if mode&0o111 != 0 {
		return true
	}

	if runtime.GOOS == "windows" {
		ext := filepath.Ext(path)
		return isWindowsExecutableExt(ext)
	}

	return false
}

func RunExternalPlugin(ctx context.Context, name string, args []string) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := UpdatePathForPlugins(); err != nil {
		logrus.Warnf("failed to update PATH environment: %v", err)
		// PATH update failure shouldn't prevent plugin execution
	}

	plugins, err := DiscoverPlugins()
	if err != nil {
		logrus.Warnf("failed to discover plugins: %v", err)
		return
	}

	var execPath string
	for _, plugin := range plugins {
		if plugin.Name == name {
			execPath = plugin.Path
			break
		}
	}

	if execPath == "" {
		return
	}

	cmd := exec.CommandContext(ctx, execPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err = cmd.Run()
	usrlocalsharelima.HandleExitError(err)
	if err == nil {
		os.Exit(0) //nolint:revive // it's intentional to call os.Exit in this function
	}
	logrus.Fatalf("external command %q failed: %v", execPath, err)
}

var descRegex = regexp.MustCompile(`<limactl-desc>(.*?)</limactl-desc>`)

func extractDescFromScript(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		logrus.Debugf("Failed to read plugin script %s: %v", path, err)
		return ""
	}

	if !strings.HasPrefix(string(content), "#!") {
		logrus.Debugf("Plugin %s: not a script file, skipping description extraction", filepath.Base(path))
		return ""
	}

	matches := descRegex.FindAllStringSubmatch(string(content), -1)
	if len(matches) == 0 {
		logrus.Debugf("Plugin %s: no <limactl-desc> found in script", filepath.Base(path))
		return ""
	}

	var descs []string
	for _, match := range matches {
		if len(match) > 1 {
			descs = append(descs, strings.TrimSpace(match[1]))
		}
	}

	if len(descs) == 0 {
		return ""
	}

	mergedDesc := descs[0]
	for _, line := range descs[1:] {
		mergedDesc += "\n" + strings.Repeat(" ", 22) + line
	}

	logrus.Debugf("Plugin %s: extracted merged description from script:\n%q", filepath.Base(path), mergedDesc)
	return mergedDesc
}

func UpdatePathForPlugins() error {
	pluginDirs := getPluginDirectories()
	newPath := strings.Join(pluginDirs, string(filepath.ListSeparator))
	return os.Setenv("PATH", newPath)
}
