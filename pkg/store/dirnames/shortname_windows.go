package dirnames

import (
	"syscall"
)

// ShortPathName return the short path name, when given a long path.
func ShortPathName(path string) (string, error) {
	p := syscall.StringToUTF16(path)
	b := p // GetShortPathName says we can reuse buffer
	n, err := syscall.GetShortPathName(&p[0], &b[0], uint32(len(b)))
	if err != nil {
		return "", err
	}
	if n > uint32(len(b)) {
		b = make([]uint16, n)
		n, err = syscall.GetShortPathName(&p[0], &b[0], uint32(len(b)))
		if err != nil {
			return "", err
		}
	}
	return syscall.UTF16ToString(b), nil
}
