/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
