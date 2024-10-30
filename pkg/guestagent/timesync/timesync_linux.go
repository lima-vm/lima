package timesync

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const rtc = "/dev/rtc"

func HasRTC() (bool, error) {
	_, err := os.Stat(rtc)
	return !errors.Is(err, os.ErrNotExist), err
}

func GetRTCTime() (t time.Time, err error) {
	f, err := os.Open(rtc)
	if err != nil {
		return
	}
	defer f.Close()
	obj, err := unix.IoctlGetRTCTime(int(f.Fd()))
	if err != nil {
		return
	}
	t = time.Date(int(obj.Year+1900), time.Month(obj.Mon+1), int(obj.Mday), int(obj.Hour), int(obj.Min), int(obj.Sec), 0, time.UTC)
	return t, nil
}

func SetSystemTime(t time.Time) error {
	v := unix.NsecToTimeval(t.UnixNano())
	return unix.Settimeofday(&v)
}
