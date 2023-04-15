package networks

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/gvisor-tap-vsock/pkg/virtualnetwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type GVisorNetstackOpts struct {
	MTU    int
	Stream bool

	Conn net.Conn

	MacAddress   string
	SSHLocalPort int
}

var (
	opts *GVisorNetstackOpts
)

func StartGVisorNetstack(ctx context.Context, gVisorOpts *GVisorNetstackOpts) {
	opts = gVisorOpts

	var protocol types.Protocol
	if opts.Stream == false {
		protocol = types.BessProtocol
	} else {
		protocol = types.QemuProtocol
	}

	// The way gvisor-tap-vsock implemented slirp is different from tradition SLIRP,
	// - GatewayIP handling all request, also answers DNS queries
	// - based on NAT configuration, gateway forwards and translates calls to host
	// Comparing this with QEMU SLIRP,
	// - DNS is equivalent to GatewayIP
	// - GatewayIP is equivalent to NAT configuration
	config := types.Configuration{
		Debug:             false,
		MTU:               opts.MTU,
		Subnet:            SlirpNetwork,
		GatewayIP:         SlirpDNS,
		GatewayMacAddress: "5a:94:ef:e4:0c:dd",
		DHCPStaticLeases: map[string]string{
			SlirpIPAddress: opts.MacAddress,
		},
		Forwards: map[string]string{
			fmt.Sprintf("127.0.0.1:%d", opts.SSHLocalPort): net.JoinHostPort(SlirpIPAddress, "22"),
		},
		DNS:              []types.Zone{},
		DNSSearchDomains: searchDomains(),
		NAT: map[string]string{
			SlirpGateway: "127.0.0.1",
		},
		GatewayVirtualIPs: []string{SlirpGateway},
		Protocol:          protocol,
	}

	groupErrs, ctx := errgroup.WithContext(ctx)
	groupErrs.Go(func() error {
		return run(ctx, groupErrs, &config)
	})
	go func() {
		err := groupErrs.Wait()
		if err != nil {
			logrus.Errorf("virtual network error: %q", err)
		}
	}()
}

func run(ctx context.Context, g *errgroup.Group, configuration *types.Configuration) error {
	vn, err := virtualnetwork.New(configuration)
	if err != nil {
		return err
	}

	if opts.Conn != nil {
		g.Go(func() error {
			if opts.Stream == false {
				return vn.AcceptBess(ctx, opts.Conn)
			}
			return vn.AcceptQemu(ctx, opts.Conn)
		})
	}

	return nil
}

func searchDomains() []string {
	if runtime.GOOS != "windows" {
		f, err := os.Open("/etc/resolv.conf")
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
	}
	return nil
}
