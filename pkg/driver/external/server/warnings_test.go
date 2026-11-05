// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"google.golang.org/grpc"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/driver/external/client"
)

type warningDriver struct {
	driver.Driver // only Validate is implemented
	err           error
}

func (d *warningDriver) Validate(_ context.Context) error {
	logrus.Info("info is not relayed")
	logrus.WithField("foo", "bar").Warn("warning is relayed")
	return d.err
}

func TestValidateRelaysWarnings(t *testing.T) {
	for name, validateErr := range map[string]error{"success": nil, "failure": errors.New("validation failed")} {
		t.Run(name, func(t *testing.T) {
			//nolint:usetesting // t.TempDir() may return a path too long for a Unix socket.
			dir, err := os.MkdirTemp("", "lima")
			assert.NilError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			socketPath := filepath.Join(dir, "drv.sock")
			var lc net.ListenConfig
			listener, err := lc.Listen(t.Context(), "unix", socketPath)
			assert.NilError(t, err)
			srv := grpc.NewServer()
			pb.RegisterDriverServer(srv, &DriverServer{driver: &warningDriver{err: validateErr}, logger: logrus.New()})
			go func() { _ = srv.Serve(listener) }()
			defer srv.Stop()

			c, err := client.NewDriverClient(socketPath, logrus.New())
			assert.NilError(t, err)
			defer c.Conn.Close()

			// The server and the client share the standard logger in this test,
			// so collect only the entries logged by the client after Validate returns.
			hook := logrustest.NewGlobal()
			defer hook.Reset()
			err = c.Validate(t.Context())
			if validateErr == nil {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, validateErr.Error())
			}

			var relayed []*logrus.Entry
			for _, e := range hook.AllEntries() {
				if e.Level == logrus.WarnLevel && e.Message == "warning is relayed" {
					relayed = append(relayed, e)
				}
			}
			// One entry is logged by the driver (server side), one is relayed by the client.
			assert.Equal(t, len(relayed), 2)
			assert.Equal(t, relayed[1].Data["foo"], "bar")
		})
	}
}
