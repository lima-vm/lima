// This file has been adapted from https://github.com/norouter/norouter/blob/v0.6.4/pkg/agent/dns/dns.go

package dns

import (
	"fmt"
	"net"
	"strings"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

// Truncate for avoiding "Parse error" from `busybox nslookup`
// https://github.com/lima-vm/lima/issues/380
const truncateSize = 512

var defaultFallbackIPs = []string{"8.8.8.8", "1.1.1.1"}

type HandlerOptions struct {
	IPv6            bool
	StaticHosts     map[string]string
	UpstreamServers []string
}

type ServerOptions struct {
	HandlerOptions
	Address string
	TCPPort int
	UDPPort int
}

type Handler struct {
	clientConfig *dns.ClientConfig
	clients      []*dns.Client
	IPv6         bool
	cname        map[string]string
	ip           map[string]net.IP
}

type Server struct {
	udp *dns.Server
	tcp *dns.Server
}

func (s *Server) Shutdown() {
	if s.udp != nil {
		_ = s.udp.Shutdown()
	}
	if s.tcp != nil {
		_ = s.tcp.Shutdown()
	}
}

func newStaticClientConfig(ips []string) (*dns.ClientConfig, error) {
	s := ``
	for _, ip := range ips {
		s += fmt.Sprintf("nameserver %s\n", ip)
	}
	r := strings.NewReader(s)
	return dns.ClientConfigFromReader(r)
}

func NewHandler(opts HandlerOptions) (dns.Handler, error) {
	cc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		logrus.WithError(err).Warnf("failed to detect system DNS, falling back to %v", defaultFallbackIPs)
		cc, err = newStaticClientConfig(defaultFallbackIPs)
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
		IPv6:         opts.IPv6,
		cname:        make(map[string]string),
		ip:           make(map[string]net.IP),
	}
	for host, address := range opts.StaticHosts {
		if ip := net.ParseIP(address); ip != nil {
			h.ip[host] = ip
		} else {
			h.cname[host] = limayaml.Cname(address)
		}
	}
	return h, nil
}

func (h *Handler) handleQuery(w dns.ResponseWriter, req *dns.Msg) {
	var (
		reply   dns.Msg
		handled bool
	)
	reply.SetReply(req)
	for _, q := range req.Question {
		hdr := dns.RR_Header{
			Name:   q.Name,
			Rrtype: q.Qtype,
			Class:  q.Qclass,
			Ttl:    5,
		}
		qtype := q.Qtype
		switch qtype {
		case dns.TypeAAAA:
			if !h.IPv6 {
				// A "correct" answer would be to set `handled = true` and return a NODATA response.
				// Unfortunately some older resolvers use a slow random source to set the transaction id.
				// This creates a problem on M1 computers, which are too fast for that implementation:
				// Both the A and AAAA queries might end up with the same id. Returning NODATA for AAAA
				// is faster, so would arrive first, and be treated as the response to the A query.
				// To avoid this, we will treat an AAAA query as an A query when IPv6 has been disabled.
				// This way it is either a valid response for an A query, or the A records will be discarded
				// by a genuine AAAA query, resulting in the desired NODATA response.
				qtype = dns.TypeA
			}
			fallthrough
		case dns.TypeCNAME, dns.TypeA:
			cname := q.Name
			seen := make(map[string]bool)
			for {
				// break cyclic definition
				if seen[cname] {
					break
				}
				if _, ok := h.cname[cname]; ok {
					seen[cname] = true
					cname = h.cname[cname]
					continue
				}
				break
			}
			var err error
			if _, ok := h.ip[cname]; !ok {
				cname, err = net.LookupCNAME(cname)
				if err != nil {
					break
				}
			}
			if cname != "" && cname != q.Name {
				hdr.Rrtype = dns.TypeCNAME
				a := &dns.CNAME{
					Hdr:    hdr,
					Target: cname,
				}
				reply.Answer = append(reply.Answer, a)
				handled = true
			}
			if qtype == dns.TypeCNAME {
				break
			}
			hdr.Name = cname
			var addrs []net.IP
			if _, ok := h.ip[cname]; ok {
				addrs = []net.IP{h.ip[cname]}
				err = nil
			} else {
				addrs, err = net.LookupIP(cname)
			}
			if err == nil && len(addrs) > 0 {
				for _, ip := range addrs {
					var a dns.RR
					ipv6 := ip.To4() == nil
					if qtype == dns.TypeA && !ipv6 {
						hdr.Rrtype = dns.TypeA
						a = &dns.A{
							Hdr: hdr,
							A:   ip.To4(),
						}
					} else if qtype == dns.TypeAAAA && ipv6 {
						hdr.Rrtype = dns.TypeAAAA
						a = &dns.AAAA{
							Hdr:  hdr,
							AAAA: ip.To16(),
						}
					} else {
						continue
					}
					reply.Answer = append(reply.Answer, a)
					handled = true
				}
			}
		case dns.TypeTXT:
			txt, err := net.LookupTXT(q.Name)
			if err == nil && len(txt) > 0 {
				for _, s := range txt {
					a := &dns.TXT{
						Hdr: hdr,
					}
					// Per RFC7208 3.3, when a TXT answer has multiple strings, the answer must be treated as
					// a single concatenated string. net.LookupTXT is pre-concatenating such answers, which
					// means we need to break it back up for this resolver to return a valid response.
					a.Txt = chunkify(s, 255)
					reply.Answer = append(reply.Answer, a)
					handled = true
				}
			}
		case dns.TypeNS:
			ns, err := net.LookupNS(q.Name)
			if err == nil && len(ns) > 0 {
				for _, s := range ns {
					if s.Host != "" {
						a := &dns.NS{
							Hdr: hdr,
							Ns:  s.Host,
						}
						reply.Answer = append(reply.Answer, a)
						handled = true
					}
				}
			}
		case dns.TypeMX:
			mx, err := net.LookupMX(q.Name)
			if err == nil && len(mx) > 0 {
				for _, s := range mx {
					if s.Host != "" {
						a := &dns.MX{
							Hdr:        hdr,
							Mx:         s.Host,
							Preference: s.Pref,
						}
						reply.Answer = append(reply.Answer, a)
						handled = true
					}
				}
			}
		case dns.TypeSRV:
			_, addrs, err := net.LookupSRV("", "", q.Name)
			if err == nil {
				hdr.Rrtype = dns.TypeSRV
				for _, addr := range addrs {
					a := &dns.SRV{
						Hdr:      hdr,
						Target:   addr.Target,
						Port:     addr.Port,
						Priority: addr.Priority,
						Weight:   addr.Weight,
					}
					reply.Answer = append(reply.Answer, a)
					handled = true
				}
			}
		}
	}
	if handled {
		reply.Truncate(truncateSize)
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
				reply.Truncate(truncateSize)
				_ = w.WriteMsg(reply)
				return
			}
		}
	}
	var reply dns.Msg
	reply.SetReply(req)
	reply.Truncate(truncateSize)
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

func Start(opts ServerOptions) (*Server, error) {
	h, err := NewHandler(opts.HandlerOptions)
	if err != nil {
		return nil, err
	}
	server := &Server{}
	if opts.UDPPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", opts.UDPPort)
		s := &dns.Server{Net: "udp", Addr: addr, Handler: h}
		server.udp = s
		go func() {
			if e := s.ListenAndServe(); e != nil {
				panic(e)
			}
		}()
	}
	if opts.TCPPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", opts.TCPPort)
		s := &dns.Server{Net: "tcp", Addr: addr, Handler: h}
		server.tcp = s
		go func() {
			if e := s.ListenAndServe(); e != nil {
				panic(e)
			}
		}()
	}
	return server, nil
}

func chunkify(buffer string, limit int) []string {
	var result []string
	for len(buffer) > 0 {
		if len(buffer) < limit {
			limit = len(buffer)
		}
		result = append(result, buffer[:limit])
		buffer = buffer[limit:]
	}
	return result
}
