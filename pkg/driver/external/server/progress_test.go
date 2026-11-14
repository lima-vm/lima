// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
	"github.com/lima-vm/lima/v2/pkg/driver/external/client"
)

type progressDriver struct {
	driver.Driver // only Start is implemented
	onProgress    func(string)
	errCh         chan error
}

func (d *progressDriver) SetStartProgressCallback(callback func(string)) {
	d.onProgress = callback
}

func (d *progressDriver) Start(_ context.Context) (chan error, error) {
	d.onProgress("waiting for the user")
	d.onProgress("done waiting")
	return d.errCh, nil
}

func startProgressServer(t *testing.T, drv *progressDriver) *client.DriverClient {
	//nolint:usetesting // t.TempDir() may return a path too long for a Unix socket.
	dir, err := os.MkdirTemp("", "lima")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "drv.sock")
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "unix", socketPath)
	assert.NilError(t, err)
	srv := grpc.NewServer()
	pb.RegisterDriverServer(srv, &DriverServer{driver: drv, logger: logrus.New()})
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(srv.Stop)

	c, err := client.NewDriverClient(socketPath, logrus.New())
	assert.NilError(t, err)
	t.Cleanup(func() { _ = c.Conn.Close() })
	return c
}

func TestStartRelaysProgress(t *testing.T) {
	drv := &progressDriver{errCh: make(chan error)}
	c := startProgressServer(t, drv)

	var progress []string
	c.SetStartProgressCallback(func(message string) {
		progress = append(progress, message)
	})
	errCh, err := c.Start(t.Context())
	assert.NilError(t, err)
	assert.DeepEqual(t, progress, []string{"waiting for the user", "done waiting"})

	// Progress reported after Start returned must not reach the client as an error.
	drv.onProgress("too late")
	close(drv.errCh)
	select {
	case err, ok := <-errCh:
		assert.Assert(t, !ok, "unexpected error: %v", err)
	case <-time.After(10 * time.Second):
		assert.Assert(t, false, "timed out waiting for the error channel to close")
	}
}

// A client that does not declare StartProgressMetadataKey treats any response
// without success as an error, so it must not receive progress.
func TestStartWithoutProgressMetadata(t *testing.T) {
	drv := &progressDriver{errCh: make(chan error)}
	c := startProgressServer(t, drv)

	stream, err := c.DriverSvc.Start(t.Context(), &emptypb.Empty{})
	assert.NilError(t, err)
	resp, err := stream.Recv()
	assert.NilError(t, err)
	assert.Assert(t, resp.Success)
	assert.Equal(t, resp.Progress, "")
	close(drv.errCh)
}
