package sockstat

import (
	"os/exec"
)

// ParseOutput parses sockstat(1)
func ParseOutput() ([]Entry, error) {
	var res []Entry

	cmd := exec.Command("sockstat", "-l", "-L", "-P", "tcp")
	_,err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return res,nil
}
