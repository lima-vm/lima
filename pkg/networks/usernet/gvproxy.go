// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/balajiv113/fd"
	"github.com/containers/gvisor-tap-vsock/pkg/transport"
	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/gvisor-tap-vsock/pkg/virtualnetwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

type GVisorNetstackOpts struct {
	MTU int

	QemuSocket string
	FdSocket   string
	Endpoint   string

	Subnet string

	Async bool

	DefaultLeases map[string]string
}

var opts *GVisorNetstackOpts

const gatewayMacAddr = "5a:94:ef:e4:0c:dd"

func StartGVisorNetstack(ctx context.Context, gVisorOpts *GVisorNetstackOpts) error {
	opts = gVisorOpts

	ip, ipNet, err := net.ParseCIDR(opts.Subnet)
	if err != nil {
		return err
	}
	gatewayIP := GatewayIP(ip)

	leases := map[string]string{}
	if opts.DefaultLeases != nil {
		for k, v := range opts.DefaultLeases {
			if ipNet.Contains(net.ParseIP(k)) {
				leases[k] = v
			}
		}
	}
	leases[gatewayIP] = gatewayMacAddr

	// The way gvisor-tap-vsock implemented slirp is different from tradition SLIRP,
	// - GatewayIP handling all request, also answers DNS queries
	// - based on NAT configuration, gateway forwards and translates calls to host
	// Comparing this with QEMU SLIRP,
	// - DNS is equivalent to GatewayIP
	// - GatewayIP is equivalent to NAT configuration
	config := types.Configuration{
		Debug:             false,
		MTU:               opts.MTU,
		Subnet:            opts.Subnet,
		GatewayIP:         gatewayIP,
		GatewayMacAddress: gatewayMacAddr,
		DHCPStaticLeases:  leases,
		Forwards:          map[string]string{},
		DNS:               []types.Zone{},
		DNSSearchDomains:  searchDomains(),
		NAT: map[string]string{
			gatewayIP: "127.0.0.1",
		},
		GatewayVirtualIPs: []string{gatewayIP},
	}

	groupErrs, ctx := errgroup.WithContext(ctx)
	err = run(ctx, groupErrs, &config)
	if err != nil {
		return err
	}
	if opts.Async {
		return err
	}
	return groupErrs.Wait()
}

func run(ctx context.Context, g *errgroup.Group, configuration *types.Configuration) error {
	vn, err := virtualnetwork.New(configuration)
	if err != nil {
		return err
	}

	ln, err := transport.Listen(fmt.Sprintf("unix://%s", opts.Endpoint))
	if err != nil {
		return err
	}

	httpServe(ctx, g, ln, muxWithExtension(vn))

	if opts.QemuSocket != "" {
		err = listenQEMU(ctx, vn)
		if err != nil {
			return err
		}
	}
	if opts.FdSocket != "" {
		err = listenFD(ctx, vn)
		if err != nil {
			return err
		}
	}
	return nil
}

func listenQEMU(ctx context.Context, vn *virtualnetwork.VirtualNetwork) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "unix", opts.QemuSocket)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		<-ctx.Done()
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				logrus.Error("QEMU accept failed", err)
			}

			go func() {
				err = vn.AcceptQemu(ctx, conn)
				if err != nil {
					logrus.Error("QEMU connection closed with error", err)
				}
				conn.Close()
			}()
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
	}()

	return nil
}

func listenFD(ctx context.Context, vn *virtualnetwork.VirtualNetwork) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "unix", opts.FdSocket)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		<-ctx.Done()
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				logrus.Error("FD accept failed", err)
				continue // since conn is nil
			}

			files, err := fd.Get(conn.(*net.UnixConn), 1, []string{"client"})
			if err != nil {
				logrus.Error("Failed to get FD via socket", err)
			}

			if len(files) != 1 {
				logrus.Error("Invalid number of fd in response", err)
			}
			fileConn, err := net.FileConn(files[0])
			if err != nil {
				logrus.Error("Error in FD Socket", err)
			}
			files[0].Close()

			go func() {
				err = vn.AcceptBess(ctx, &UDPFileConn{Conn: fileConn})
				if err != nil {
					logrus.Error("FD connection closed with error", err)
				}
				fileConn.Close()
			}()
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
	}()

	return nil
}

func httpServe(ctx context.Context, g *errgroup.Group, ln net.Listener, mux http.Handler) {
	s := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	g.Go(func() error {
		<-ctx.Done()
		return s.Close()
	})
	g.Go(func() error {
		err := s.Serve(ln)
		if err != nil {
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		}
		return nil
	})
}

func muxWithExtension(n *virtualnetwork.VirtualNetwork) *http.ServeMux {
	m := n.Mux()
	m.HandleFunc("/extension/wait-ssh-server", func(w http.ResponseWriter, r *http.Request) {
		ip := r.URL.Query().Get("ip")
		if net.ParseIP(ip) == nil {
			msg := fmt.Sprintf("invalid ip address: %s", ip)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		port16, err := strconv.ParseUint(r.URL.Query().Get("port"), 10, 16)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", uint16(port16)))

		user := r.URL.Query().Get("user")
		if user == "" {
			msg := "user query parameter is required"
			http.Error(w, msg, http.StatusBadRequest)
			return
		}

		timeoutSeconds := 10
		if timeoutString := r.URL.Query().Get("timeout"); timeoutString != "" {
			timeout16, err := strconv.ParseUint(timeoutString, 10, 16)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			timeoutSeconds = int(timeout16)
		}
		dialContext := func(ctx context.Context) (net.Conn, error) {
			return n.DialContextTCP(ctx, addr)
		}
		// Wait until the port is available.
		if err = sshutil.WaitSSHReady(r.Context(), dialContext, addr, user, timeoutSeconds); err != nil {
			http.Error(w, err.Error(), http.StatusRequestTimeout)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	return m
}

func searchDomains() []string {
	if runtime.GOOS != "windows" {
		return resolveSearchDomain("/etc/resolv.conf")
	}
	return nil
}

func resolveSearchDomain(file string) []string {
	f, err := os.Open(file)
	if err != nil {
		logrus.Errorf("open file error: %v", err)
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	searchPrefix := "search "
	for sc.Scan() {
		if after, ok := strings.CutPrefix(sc.Text(), searchPrefix); ok {
			searchDomains := strings.Split(after, " ")
			logrus.Debugf("Using search domains: %v", searchDomains)
			return searchDomains
		}
	}
	if err := sc.Err(); err != nil {
		logrus.Errorf("scan file error: %v", err)
		return nil
	}
	return nil
}
