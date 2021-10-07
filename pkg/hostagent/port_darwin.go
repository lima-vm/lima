package hostagent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/norouter/norouter/pkg/agent/bicopy"
	"github.com/sirupsen/logrus"
)

// forwardTCP is not thread-safe
func forwardTCP(ctx context.Context, l *logrus.Logger, sshConfig *ssh.SSHConfig, port int, local, remote string, cancel bool) error {
	localIPStr, localPortStr, err := net.SplitHostPort(local)
	if err != nil {
		return err
	}
	localIP := net.ParseIP(localIPStr)
	localPort, err := strconv.Atoi(localPortStr)
	if err != nil {
		return err
	}

	if !net.ParseIP("127.0.0.1").Equal(localIP) || localPort >= 1024 {
		return forwardSSH(ctx, sshConfig, port, local, remote, cancel)
	}

	// on macOS, listening on 127.0.0.1:80 requires root while 0.0.0.0:80 does not require root.
	// https://twitter.com/_AkihiroSuda_/status/1403403845842075648
	//
	// We use "pseudoloopback" forwarder that listens on 0.0.0.0:80 but rejects connections from non-loopback src IP.
	l.Debugf("using pseudoloopback port forwarder for %q", local)

	if cancel {
		plf, ok := pseudoLoopbackForwarders[local]
		if ok {
			localUnix := plf.unixAddr.Name
			_ = plf.Close()
			delete(pseudoLoopbackForwarders, local)
			if err := forwardSSH(ctx, sshConfig, port, localUnix, remote, cancel); err != nil {
				return err
			}
		} else {
			l.Warnf("forwarding for %q seems already cancelled?", local)
		}
		return nil
	}

	localUnixDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("lima-psl-%s-%d-", localIP, localPort))
	if err != nil {
		return err
	}
	localUnix := filepath.Join(localUnixDir, "sock")
	l.Debugf("forwarding %q to %q", localUnix, remote)
	if err := forwardSSH(ctx, sshConfig, port, localUnix, remote, cancel); err != nil {
		if removeErr := os.RemoveAll(localUnixDir); removeErr != nil {
			l.WithError(removeErr).Warnf("failed to remove %q", removeErr)
		}
		return err
	}
	plf, err := newPseudoLoopbackForwarder(l, localPort, localUnix)
	if err != nil {
		if removeErr := os.RemoveAll(localUnixDir); removeErr != nil {
			l.WithError(removeErr).Warnf("failed to remove %q", removeErr)
		}
		if cancelErr := forwardSSH(ctx, sshConfig, port, localUnix, remote, true); cancelErr != nil {
			l.WithError(cancelErr).Warnf("failed to cancel forwarding %q to %q", localUnix, remote)
		}
		return err
	}
	plf.onClose = func() error {
		return os.RemoveAll(localUnixDir)
	}
	pseudoLoopbackForwarders[local] = plf
	go func() {
		if plfErr := plf.Serve(); plfErr != nil {
			l.WithError(plfErr).Warning("pseudoloopback forwarder crashed")
		}
	}()
	return nil
}

var pseudoLoopbackForwarders = make(map[string]*pseudoLoopbackForwarder)

type pseudoLoopbackForwarder struct {
	l        *logrus.Logger
	ln       *net.TCPListener
	unixAddr *net.UnixAddr
	onClose  func() error
}

func newPseudoLoopbackForwarder(l *logrus.Logger, localPort int, unixSock string) (*pseudoLoopbackForwarder, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", unixSock)
	if err != nil {
		return nil, err
	}

	lnAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("0.0.0.0:%d", localPort))
	if err != nil {
		return nil, err
	}
	ln, err := net.ListenTCP("tcp4", lnAddr)
	if err != nil {
		return nil, err
	}

	plf := &pseudoLoopbackForwarder{
		l:        l,
		ln:       ln,
		unixAddr: unixAddr,
	}

	return plf, nil
}

func (plf *pseudoLoopbackForwarder) Serve() error {
	defer plf.ln.Close()
	for {
		ac, err := plf.ln.AcceptTCP()
		if err != nil {
			return err
		}
		remoteAddr := ac.RemoteAddr().String() // ip:port
		remoteAddrIP, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			plf.l.WithError(err).Debugf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q (unparsable)", remoteAddr)
			ac.Close()
			continue
		}
		if remoteAddrIP != "127.0.0.1" {
			plf.l.WithError(err).Debugf("pseudoloopback forwarder: rejecting non-loopback remoteAddr %q", remoteAddr)
			ac.Close()
			continue
		}
		go func(ac *net.TCPConn) {
			if fErr := plf.forward(ac); fErr != nil {
				plf.l.Error(fErr)
			}
		}(ac)
	}
}

func (plf *pseudoLoopbackForwarder) forward(ac *net.TCPConn) error {
	defer ac.Close()
	unixConn, err := net.DialUnix("unix", nil, plf.unixAddr)
	if err != nil {
		return err
	}
	defer unixConn.Close()
	bicopy.Bicopy(ac, unixConn, nil)
	return nil
}

func (plf *pseudoLoopbackForwarder) Close() error {
	_ = plf.ln.Close()
	return plf.onClose()
}
