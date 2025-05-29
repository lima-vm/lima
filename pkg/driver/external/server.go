// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package external

import (
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/lima-vm/lima/pkg/driver"
)

type Server struct {
	UnimplementedDriverServer
	driver driver.Driver
	logger *logrus.Logger
}

func Serve(driver driver.Driver) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.Infof("Starting external driver server for %s", driver.Name())

	pipeConn := &PipeConn{
		Reader: os.Stdin,
		Writer: os.Stdout,
	}

	listener := NewPipeListener(pipeConn)

	kaProps := keepalive.ServerParameters{
		Time:    10 * time.Second,
		Timeout: 20 * time.Second,
	}

	kaPolicy := keepalive.EnforcementPolicy{
		MinTime:             2 * time.Second,
		PermitWithoutStream: true,
	}

	server := grpc.NewServer(
		grpc.KeepaliveParams(kaProps),
		grpc.KeepaliveEnforcementPolicy(kaPolicy),
	)

	RegisterDriverServer(server, &Server{
		driver: driver,
		logger: logger,
	})

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		logger.Info("Received shutdown signal, stopping server...")
		server.Stop()
		os.Exit(0)
	}()

	logger.Info("Server starting...")
	if err := server.Serve(listener); err != nil {
		logger.Fatalf("Failed to serve: %v", err)
	}
}

type PipeConn struct {
	Reader io.Reader
	Writer io.Writer
}

func (p *PipeConn) Read(b []byte) (n int, err error) {
	return p.Reader.Read(b)
}

func (p *PipeConn) Write(b []byte) (n int, err error) {
	return p.Writer.Write(b)
}

func (p *PipeConn) Close() error {
	return nil
}

func (p *PipeConn) LocalAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) RemoteAddr() net.Addr {
	return pipeAddr{}
}

func (p *PipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *PipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

type PipeListener struct {
	conn     net.Conn
	connSent bool
	mu       sync.Mutex
	closed   bool
}

func NewPipeListener(conn net.Conn) *PipeListener {
	return &PipeListener{
		conn:     conn,
		connSent: false,
		closed:   false,
	}
}

func (l *PipeListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, net.ErrClosed
	}

	if l.connSent {
		select {}
	}

	l.connSent = true
	return l.conn, nil
}

func (l *PipeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.closed {
		l.closed = true
	}
	return nil
}

func (l *PipeListener) Addr() net.Addr {
	return pipeAddr{}
}
