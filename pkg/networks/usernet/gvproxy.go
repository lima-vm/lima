package usernet

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lima-vm/lima/pkg/networks"

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

	Async bool
}

var (
	opts *GVisorNetstackOpts
)

func StartGVisorNetstack(ctx context.Context, gVisorOpts *GVisorNetstackOpts) error {
	opts = gVisorOpts

	// The way gvisor-tap-vsock implemented slirp is different from tradition SLIRP,
	// - GatewayIP handling all request, also answers DNS queries
	// - based on NAT configuration, gateway forwards and translates calls to host
	// Comparing this with QEMU SLIRP,
	// - DNS is equivalent to GatewayIP
	// - GatewayIP is equivalent to NAT configuration
	config := types.Configuration{
		Debug:             false,
		MTU:               opts.MTU,
		Subnet:            networks.SlirpNetwork,
		GatewayIP:         networks.SlirpDNS,
		GatewayMacAddress: "5a:94:ef:e4:0c:dd",
		DHCPStaticLeases: map[string]string{
			networks.SlirpGateway: "5a:94:ef:e4:0c:df",
		},
		Forwards:         map[string]string{},
		DNS:              []types.Zone{},
		DNSSearchDomains: searchDomains(),
		NAT: map[string]string{
			networks.SlirpGateway: "127.0.0.1",
		},
		GatewayVirtualIPs: []string{networks.SlirpGateway},
	}

	groupErrs, ctx := errgroup.WithContext(ctx)
	err := run(ctx, groupErrs, &config)
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
	httpServe(ctx, g, ln, vn.Mux())

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
	listener, err := net.Listen("unix", opts.QemuSocket)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
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
	listener, err := net.Listen("unix", opts.FdSocket)
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				logrus.Error("FD accept failed", err)
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
				err = vn.AcceptBess(ctx, fileConn)
				if err != nil {
					logrus.Error("FD connection closed with error", err)
				}
				defer fileConn.Close()
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
	g.Go(func() error {
		<-ctx.Done()
		return ln.Close()
	})
	g.Go(func() error {
		s := &http.Server{
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		err := s.Serve(ln)
		if err != nil {
			if err != http.ErrServerClosed {
				return err
			}
			return err
		}
		return nil
	})
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
