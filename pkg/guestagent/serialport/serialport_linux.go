package serialport

import (
	"os"

	"golang.org/x/sys/unix"
)

func openPort(name string) (p *Port, err error) {
	f, err := os.OpenFile(name, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0o666)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && f != nil {
			f.Close()
		}
	}()

	fd := f.Fd()
	if err = unix.SetNonblock(int(fd), false); err != nil {
		return nil, err
	}

	return &Port{f: f}, nil
}

type Port struct {
	f *os.File
}

func (p *Port) Read(b []byte) (n int, err error) {
	return p.f.Read(b)
}

func (p *Port) Write(b []byte) (n int, err error) {
	return p.f.Write(b)
}

func (p *Port) Close() (err error) {
	return p.f.Close()
}
