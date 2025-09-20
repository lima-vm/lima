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
	m.HandleFunc("/extension/wait_port", func(w http.ResponseWriter, r *http.Request) {
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
		port := uint16(port16)
		addr := fmt.Sprintf("%s:%d", ip, port)

		timeoutSeconds := 10
		if timeoutString := r.URL.Query().Get("timeout"); timeoutString != "" {
			timeout16, err := strconv.ParseUint(timeoutString, 10, 16)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			timeoutSeconds = int(timeout16)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		// Wait until the port is available.
		for {
			conn, err := n.DialContextTCP(ctx, addr)
			if err == nil {
				conn.Close()
				logrus.Debugf("Port is available on %s", addr)
				w.WriteHeader(http.StatusOK)
				break
			}
			select {
			case <-ctx.Done():
				msg := fmt.Sprintf("timed out waiting for port to become available on %s", addr)
				logrus.Warn(msg)
				http.Error(w, msg, http.StatusRequestTimeout)
				return
			default:
			}
			logrus.Debugf("Waiting for port to become available on %s", addr)
			time.Sleep(1 * time.Second)
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
		if strings.HasPrefix(sc.Text(), searchPrefix) {
			searchDomains := strings.Split(strings.TrimPrefix(sc.Text(), searchPrefix), " ")
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
