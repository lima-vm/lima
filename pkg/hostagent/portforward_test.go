// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	guestagentclient "github.com/lima-vm/lima/v2/pkg/guestagent/api/client"
	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/portfwd"
)

// invalidIPGuestService streams one port event whose IP is not an IP address.
// A compromised guest can report an arbitrary string there; this one carries
// ANSI escapes that would rewrite the operator's terminal if it reached
// PortForwardEvent.GuestAddr.
type invalidIPGuestService struct {
	api.UnimplementedGuestServiceServer
}

func (*invalidIPGuestService) GetInfo(context.Context, *emptypb.Empty) (*api.Info, error) {
	return &api.Info{}, nil
}

func (*invalidIPGuestService) GetEvents(_ *emptypb.Empty, stream api.GuestService_GetEventsServer) error {
	return stream.Send(&api.Event{
		AddedLocalPorts: []*api.IPPort{{Ip: "1.2.3.4\x1b[2Kspoofed\x07", Port: 8080, Protocol: "tcp"}},
	})
}

func TestProcessGuestAgentEventsRejectsInvalidIP(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(guestagentclient.NewCredentials()))
	api.RegisterGuestServiceServer(server, &invalidIPGuestService{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	client, err := guestagentclient.NewGuestAgentClient(listener.DialContext)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	a := &HostAgent{guestAgentAliveCh: make(chan struct{})}
	err = a.processGuestAgentEvents(t.Context(), client)
	assert.ErrorContains(t, err, "invalid IP")
}

type shutdownGuestService struct {
	api.UnimplementedGuestServiceServer
	err error
}

func (*shutdownGuestService) GetInfo(context.Context, *emptypb.Empty) (*api.Info, error) {
	return &api.Info{}, nil
}

func (s *shutdownGuestService) GetEvents(_ *emptypb.Empty, _ api.GuestService_GetEventsServer) error {
	return s.err
}

func TestIsExpectedShutdownErr(t *testing.T) {
	ctx := t.Context()
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	a := &HostAgent{}

	// When not stopping
	assert.Assert(t, a.isExpectedShutdownErr(ctx, nil))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, context.Canceled))
	assert.Assert(t, a.isExpectedShutdownErr(canceledCtx, io.EOF))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, status.Error(codes.Canceled, "canceled")))
	assert.Assert(t, !a.isExpectedShutdownErr(ctx, io.EOF))
	assert.Assert(t, !a.isExpectedShutdownErr(ctx, net.ErrClosed))
	assert.Assert(t, !a.isExpectedShutdownErr(ctx, status.Error(codes.Unavailable, "error reading from server: EOF")))
	assert.Assert(t, !a.isExpectedShutdownErr(ctx, errors.New("arbitrary error")))

	// When stopping
	a.stopping.Store(true)
	assert.Assert(t, a.isExpectedShutdownErr(ctx, nil))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, context.Canceled))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, io.EOF))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, net.ErrClosed))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, &net.OpError{Op: "read", Err: errors.New("closed")}))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, status.Error(codes.Unavailable, "error reading from server: EOF")))
	assert.Assert(t, a.isExpectedShutdownErr(ctx, status.Error(codes.Canceled, "transport is closing")))
	assert.Assert(t, !a.isExpectedShutdownErr(ctx, errors.New("arbitrary error")))
}

func TestGetOrCreateClient_Stopping(t *testing.T) {
	a := &HostAgent{}
	a.stopping.Store(true)
	client, err := a.getOrCreateClient(t.Context())
	assert.Assert(t, client == nil)
	assert.Assert(t, errors.Is(err, context.Canceled))
}

func TestProcessGuestAgentEvents_ShutdownSuppressed(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(guestagentclient.NewCredentials()))
	svc := &shutdownGuestService{
		err: status.Error(codes.Unavailable, "error reading from server: EOF"),
	}
	api.RegisterGuestServiceServer(server, svc)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	client, err := guestagentclient.NewGuestAgentClient(listener.DialContext)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	a := &HostAgent{guestAgentAliveCh: make(chan struct{})}
	a.stopping.Store(true)

	err = a.processGuestAgentEvents(t.Context(), client)
	assert.Assert(t, a.isExpectedShutdownErr(t.Context(), err))
}

type blockingGuestService struct {
	api.UnimplementedGuestServiceServer
	started chan struct{}
}

func (*blockingGuestService) GetInfo(context.Context, *emptypb.Empty) (*api.Info, error) {
	return &api.Info{}, nil
}

func (s *blockingGuestService) GetEvents(_ *emptypb.Empty, stream api.GuestService_GetEventsServer) error {
	select {
	case <-s.started:
	default:
		close(s.started)
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

type stubWatcherDriver struct {
	driver.Driver
	connFn func(context.Context) (net.Conn, error)
}

func (stubWatcherDriver) Info(context.Context) driver.Info {
	return driver.Info{Features: driver.DriverFeatures{SkipSocketForwarding: true}}
}

func (stubWatcherDriver) ForwardGuestAgent(context.Context) bool {
	return false
}

func (d stubWatcherDriver) GuestAgentConn(ctx context.Context) (net.Conn, string, error) {
	if d.connFn != nil {
		c, err := d.connFn(ctx)
		return c, "test", err
	}
	return nil, "", nil
}

type logCaptureHook struct {
	mu      sync.Mutex
	entries []*logrus.Entry
}

func (*logCaptureHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel, logrus.ErrorLevel}
}

func (h *logCaptureHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, entry)
	return nil
}

func TestWatchGuestAgentEvents_ShutdownSuppressed(t *testing.T) {
	hook := &logCaptureHook{}
	origHooks := logrus.StandardLogger().Hooks
	logrus.AddHook(hook)
	t.Cleanup(func() {
		logrus.StandardLogger().Hooks = origHooks
	})

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.Creds(guestagentclient.NewCredentials()))
	svc := &blockingGuestService{started: make(chan struct{})}
	api.RegisterGuestServiceServer(server, svc)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	a := &HostAgent{
		driver: stubWatcherDriver{
			connFn: func(context.Context) (net.Conn, error) {
				return listener.Dial()
			},
		},
		guestAgentAliveCh: make(chan struct{}),
		grpcPortForwarder: portfwd.NewPortForwarder(nil, false, false, func(*events.PortForwardEvent) {}),
		instConfig:        &limatype.LimaYAML{},
	}

	ctx := t.Context()
	done := make(chan struct{})
	go func() {
		a.watchGuestAgentEvents(ctx)
		close(done)
	}()

	select {
	case <-svc.started:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "timed out waiting for guest agent stream to start")
	}

	err := a.close()
	assert.NilError(t, err)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		assert.Assert(t, false, "watchGuestAgentEvents did not exit promptly on shutdown")
	}

	hook.mu.Lock()
	defer hook.mu.Unlock()
	for _, entry := range hook.entries {
		assert.Assert(t, !strings.Contains(entry.Message, "guest agent events closed unexpectedly"), "unexpected warning: %s", entry.Message)
		assert.Assert(t, !strings.Contains(entry.Message, "connection to the guest agent was closed unexpectedly"), "unexpected warning: %s", entry.Message)
	}
}
