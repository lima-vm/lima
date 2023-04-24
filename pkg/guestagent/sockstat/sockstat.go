package sockstat

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

type Kind = string

const (
	TCP4 Kind = "tcp4"
	TCP6 Kind = "tcp6"
	// TODO: "udp4", "udp6"
)

type State = string

const (
	Listen State = "listen"
)

type Entry struct {
	Kind  Kind   `json:"kind"`
	IP    net.IP `json:"ip"`
	Port  uint16 `json:"port"`
	State State  `json:"state"`
}

func Parse(r io.Reader, state State) ([]Entry, error) {
	var entries []Entry
	sc := bufio.NewScanner(r)

	fieldNames := make(map[string]int)
	for i := 0; sc.Scan(); i++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch i {
		case 0:
			for j := 0; j < len(fields); j++ {
				fieldNames[fields[j]] = j
			}
			if _, ok := fieldNames["PROTO"]; !ok {
				return nil, fmt.Errorf("field \"PROTO\" not found")
			}

		default:
			proto := fields[fieldNames["PROTO"]]
			var kind Kind
			switch proto {
			case "tcp4":
				kind = TCP4
			case "tcp6":
				kind = TCP6
			default:
				return entries, fmt.Errorf("unknown protocol: %s", proto)
			}
			localAddress := fields[fieldNames["PROTO"]+1]
			ip, port, err := ParseAddress(localAddress, kind)
			if err != nil {
				return entries, err
			}
			ent := Entry{
				Kind:  kind,
				IP:    ip,
				Port:  port,
				State: state,
			}
			entries = append(entries, ent)
		}
	}

	if err := sc.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}

/*
Linux:
USER     PROCESS              PID      PROTO  SOURCE ADDRESS            FOREIGN ADDRESS           STATE
root     systemd              1        tcp6   :::22                     :::*                      LISTEN
root     sshd                 1580     tcp6   :::22                     :::*                      LISTEN

FreeBSD:
USER     COMMAND    PID   FD  PROTO  LOCAL ADDRESS         FOREIGN ADDRESS
root     sshd         831 3   tcp6   *:22                  *:*
root     sshd         831 4   tcp4   *:22                  *:*
*/

func ParseAddress(s string, kind Kind) (net.IP, uint16, error) {
	split := strings.SplitN(s, ":", 2)
	if len(split) != 2 {
		return nil, 0, fmt.Errorf("unparsable address %q", s)
	}

	host := split[0]
	if host == "*" {
		switch kind {
		case TCP4:
			host = "0.0.0.0"
		case TCP6:
			host = "::"
		}
	}
	ip := net.ParseIP(host)

	port64, err := strconv.ParseUint(split[1], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("unparsable address %q: unparsable port %q", s, split[1])
	}
	port := uint16(port64)

	return ip, port, nil
}
