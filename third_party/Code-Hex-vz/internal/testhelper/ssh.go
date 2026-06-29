package testhelper

import (
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func NewSshConfig(username, password string) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

func NewSshClient(conn net.Conn, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func SetKeepAlive(t *testing.T, session *ssh.Session) {
	t.Helper()
	go func() {
		for range time.Tick(5 * time.Second) {
			_, err := session.SendRequest("keepalive@codehex.vz", true, nil)
			if err != nil && err != io.EOF {
				t.Logf("failed to send keep-alive request: %v", err)
				return
			}
		}
	}()
}
