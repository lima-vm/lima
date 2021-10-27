package procnetunix

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type State = int

const (
	StateUnconnected   State = 1
	StateConnecting    State = 2
	StateConnected     State = 3
	StateDisconnecting State = 4
)

type Entry struct {
	Path  string `json:"path"`
	State State  `json:"state"`
}

func Parse(r io.Reader) ([]Entry, error) {
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
			if _, ok := fieldNames["Path"]; !ok {
				return nil, fmt.Errorf("field \"Path\" not found")
			}
			if _, ok := fieldNames["St"]; !ok {
				return nil, fmt.Errorf("field \"St\" not found")
			}

		default:
			if len(fields) <= fieldNames["Path"] {
				continue
			}
			path := fields[fieldNames["Path"]]
			stStr := fields[fieldNames["St"]]
			st, err := strconv.ParseUint(stStr, 16, 8)
			if err != nil {
				return entries, err
			}

			ent := Entry{
				Path:  path,
				State: int(st),
			}
			entries = append(entries, ent)
		}
	}

	if err := sc.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}
