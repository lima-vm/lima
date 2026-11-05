// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
)

// warningCollector is a logrus hook that collects warnings.
type warningCollector struct {
	mu       sync.Mutex
	warnings []pb.Warning
}

func (c *warningCollector) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel}
}

func (c *warningCollector) Fire(entry *logrus.Entry) error {
	w := pb.Warning{Message: entry.Message}
	if len(entry.Data) > 0 {
		w.Fields = make(map[string]string, len(entry.Data))
		for k, v := range entry.Data {
			w.Fields[k] = fmt.Sprint(v)
		}
	}
	c.mu.Lock()
	c.warnings = append(c.warnings, w)
	c.mu.Unlock()
	return nil
}

// captureWarningsMu serializes captureWarnings, as logrus hooks are global to the logger.
var captureWarningsMu sync.Mutex

// captureWarnings runs fn while collecting the warnings logged via the standard logrus logger,
// and sets them to the gRPC trailer, so that the client can print them on limactl output.
func captureWarnings(ctx context.Context, logger *logrus.Logger, fn func() error) error {
	captureWarningsMu.Lock()
	defer captureWarningsMu.Unlock()

	std := logrus.StandardLogger()
	collector := &warningCollector{}
	hooks := make(logrus.LevelHooks)
	for level, levelHooks := range std.Hooks {
		hooks[level] = append([]logrus.Hook(nil), levelHooks...)
	}
	hooks.Add(collector)
	origHooks := std.ReplaceHooks(hooks)
	err := fn()
	std.ReplaceHooks(origHooks)

	if len(collector.warnings) > 0 {
		md := metadata.MD{}
		for _, w := range collector.warnings {
			b, err := json.Marshal(w)
			if err != nil {
				logger.WithError(err).Warn("Failed to marshal a warning")
				continue
			}
			md.Append(pb.WarningsTrailerKey, string(b))
		}
		if setErr := grpc.SetTrailer(ctx, md); setErr != nil {
			logger.WithError(setErr).Warn("Failed to set the warnings to the gRPC trailer")
		}
	}
	return err
}
