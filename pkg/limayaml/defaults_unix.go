//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func hostTimeZone() string {
	if tzBytes, err := os.ReadFile("/etc/timezone"); err == nil {
		if tz := strings.TrimSpace(string(tzBytes)); tz != "" {
			if _, err := time.LoadLocation(tz); err != nil {
				logrus.Warnf("invalid timezone found in /etc/timezone: %v", err)
			} else {
				return tz
			}
		}
	}

	if zoneinfoFile, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if tz, err := extractTZFromPath(zoneinfoFile); err != nil {
			logrus.Warnf("failed to extract timezone from %s: %v", zoneinfoFile, err)
		} else {
			return tz
		}
	}

	logrus.Warn("unable to determine host timezone, falling back to default value")
	return ""
}

func extractTZFromPath(zoneinfoFile string) (string, error) {
	if zoneinfoFile == "" {
		return "", errors.New("invalid zoneinfo file path")
	}

	if _, err := os.Stat(zoneinfoFile); os.IsNotExist(err) {
		return "", fmt.Errorf("zoneinfo file does not exist: %s", zoneinfoFile)
	}

	for dir := filepath.Dir(zoneinfoFile); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "Etc", "UTC")); err == nil {
			return filepath.Rel(dir, zoneinfoFile)
		}
	}

	return "", errors.New("timezone base directory not found")
}
