// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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

func GetRTCTime() (time.Time, error) {
	f, err := os.Open(rtc)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	obj, err := unix.IoctlGetRTCTime(int(f.Fd()))
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(int(obj.Year+1900), time.Month(obj.Mon+1), int(obj.Mday), int(obj.Hour), int(obj.Min), int(obj.Sec), 0, time.UTC), nil
}

func SetSystemTime(t time.Time) error {
	v := unix.NsecToTimeval(t.UnixNano())
	return unix.Settimeofday(&v)
}
