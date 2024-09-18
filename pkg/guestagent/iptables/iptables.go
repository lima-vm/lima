package iptables

import (
	"bytes"
	"errors"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	TCP  bool
	IP   net.IP
	Port int
}

// This regex can detect a line in the iptables added by portmap to do the
// forwarding. The following two are examples of lines (notice that one has the
// destination IP and the other does not):
//
//	-A CNI-DN-2e2f8d5b91929ef9fc152 -d 127.0.0.1/32 -p tcp -m tcp --dport 8081 -j DNAT --to-destination 10.4.0.7:80
//	-A CNI-DN-04579c7bb67f4c3f6cca0 -p tcp -m tcp --dport 8082 -j DNAT --to-destination 10.4.0.10:80
//
// The -A on the front is to amend the rule that was already created. portmap
// ensures the rule is created before creating this line so it is always -A.
// CNI-DN- is the prefix used for rule for an individual container.
// -d is followed by the IP address. The regular expression looks for a valid
// ipv4 IP address. We need to detect this IP.
// --dport is the destination port. We need to detect this port
// -j DNAT this tells us it's the line doing the port forwarding.
var findPortRegex = regexp.MustCompile(`-A\s+CNI-DN-\w*\s+(?:-d ((?:\b25[0-5]|\b2[0-4][0-9]|\b[01]?[0-9][0-9]?)(?:\.(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}))?(?:/32\s+)?-p (tcp)?.*--dport (\d+) -j DNAT`)

func GetPorts() ([]Entry, error) {
	// TODO: add support for ipv6

	// Detect the location of iptables. If it is not installed skip the lookup
	// and return no results. The lookup is performed on each run so that the
	// agent does not need to be started to detect if iptables was installed
	// after the agent is already running.
	pth, err := exec.LookPath("iptables")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	res, err := listNATRules(pth)
	if err != nil {
		return nil, err
	}

	pts, err := parsePortsFromRules(res)
	if err != nil {
		return nil, err
	}

	return checkPortsOpen(pts)
}

func parsePortsFromRules(rules []string) ([]Entry, error) {
	var entries []Entry
	for _, rule := range rules {
		if found := findPortRegex.FindStringSubmatch(rule); found != nil {
			if len(found) == 4 {
				port, err := strconv.Atoi(found[3])
				if err != nil {
					return nil, err
				}

				istcp := found[2] == "tcp"

				// When no IP is present the rule applies to all interfaces.
				ip := found[1]
				if ip == "" {
					ip = "0.0.0.0"
				}
				ent := Entry{
					IP:   net.ParseIP(ip),
					Port: port,
					TCP:  istcp,
				}
				entries = append(entries, ent)
			}
		}
	}

	return entries, nil
}

// listNATRules performs the lookup with iptables and returns the raw rules
// Note, this does not use github.com/coreos/go-iptables (a transitive dependency
// of lima) because that package would require multiple calls to iptables. This
// function does everything in a single call.
func listNATRules(pth string) ([]string, error) {
	args := []string{pth, "-t", "nat", "-S"}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Cmd{
		Path:   pth,
		Args:   args,
		Stdout: &stdout,
		Stderr: &stderr,
	}
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// turn the output into a rule per line.
	rules := strings.Split(stdout.String(), "\n")
	if len(rules) > 0 && rules[len(rules)-1] == "" {
		rules = rules[:len(rules)-1]
	}

	return rules, nil
}

func checkPortsOpen(pts []Entry) ([]Entry, error) {
	var entries []Entry
	for _, pt := range pts {
		if pt.TCP {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(pt.IP.String(), strconv.Itoa(pt.Port)), time.Second)
			if err == nil && conn != nil {
				conn.Close()
				entries = append(entries, pt)
			}
		} else {
			entries = append(entries, pt)
		}
	}

	return entries, nil
}
