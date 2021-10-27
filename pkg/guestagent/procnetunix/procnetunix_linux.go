package procnetunix

import "os"

// ParseFile parses /proc/net/unix
func ParseFile() ([]Entry, error) {
	r, err := os.Open("/proc/net/unix")
	if err != nil {
		return []Entry{}, err
	}
	defer r.Close()
	return Parse(r)
}
