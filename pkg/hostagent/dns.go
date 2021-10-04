// This file has been adapted from https://github.com/norouter/norouter/blob/v0.6.4/pkg/agent/dns/dns.go

package hostagent

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	clientConfig *dns.ClientConfig
	clients      []*dns.Client
}

func newStaticClientConfig(ips []net.IP) (*dns.ClientConfig, error) {
	s := ``
	for _, ip := range ips {
		s += fmt.Sprintf("nameserver %s\n", ip.String())
	}
	r := strings.NewReader(s)
	return dns.ClientConfigFromReader(r)
}

func newHandler() (dns.Handler, error) {
	cc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		fallbackIPs := []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("1.1.1.1")}
		logrus.WithError(err).Warnf("failed to detect system DNS, falling back to %v", fallbackIPs)
		cc, err = newStaticClientConfig(fallbackIPs)
		if err != nil {
			return nil, err
		}
	}
	clients := []*dns.Client{
		{}, // UDP
		{Net: "tcp"},
	}
	h := &Handler{
		clientConfig: cc,
		clients:      clients,
	}
	return h, nil
}

func (h *Handler) handleQuery(w dns.ResponseWriter, req *dns.Msg) {
	var (
		reply   dns.Msg
		handled bool
	)
	reply.SetReply(req)
	for _, q := range reply.Question {
		switch q.Qtype {
		case dns.TypeA:
			addrs, err := net.LookupIP(q.Name)
			if err == nil && len(addrs) > 0 {
				for _, ip := range addrs {
					if ip.To4() != nil {
						a := &dns.A{
							Hdr: dns.RR_Header{
								Name:   q.Name,
								Rrtype: dns.TypeA,
								Class:  dns.ClassINET,
							},
							A: ip.To4(),
						}
						reply.Answer = append(reply.Answer, a)
						handled = true
						break
					}
				}
			}
		}
	}
	if handled {
		_ = w.WriteMsg(&reply)
		return
	}
	h.handleDefault(w, req)
}

func (h *Handler) handleDefault(w dns.ResponseWriter, req *dns.Msg) {
	for _, client := range h.clients {
		for _, srv := range h.clientConfig.Servers {
			addr := fmt.Sprintf("%s:%s", srv, h.clientConfig.Port)
			reply, _, err := client.Exchange(req, addr)
			if err == nil {
				_ = w.WriteMsg(reply)
				return
			}
		}
	}
	var reply dns.Msg
	reply.SetReply(req)
	_ = w.WriteMsg(&reply)
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	switch req.Opcode {
	case dns.OpcodeQuery:
		h.handleQuery(w, req)
	default:
		h.handleDefault(w, req)
	}
}

func (a *HostAgent) StartDNS() (*dns.Server, error) {
	h, err := newHandler()
	if err != nil {
		panic(err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", a.udpDNSLocalPort)
	server := &dns.Server{Net: "udp", Addr: addr, Handler: h}
	go func() {
		if e := server.ListenAndServe(); e != nil {
			panic(e)
		}
	}()
	return server, nil
}
