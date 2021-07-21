package smbpasswd

import (
	"fmt"
	"io"
	"os/user"
	"strconv"
	"time"

	"golang.org/x/crypto/md4" // nolint:staticcheck
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func UTF16LE(s string) (string, error) {
	encoding := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	transformer := encoding.NewEncoder()
	res, _, err := transform.String(transformer, s)
	return res, err
}

func NTHash(s string) (string, error) {
	utf16le, err := UTF16LE(s)
	if err != nil {
		return "", err
	}
	h := md4.New()
	if _, err := io.WriteString(h, utf16le); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	res := fmt.Sprintf("%X", sum)
	return res, nil
}

func SMBPasswd(username string, uid int, plainPassword string, lct time.Time) (string, error) {
	nthash, err := NTHash(plainPassword)
	if err != nil {
		return "", err
	}
	res := fmt.Sprintf("%s:%d:XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:%s:[U          ]:LCT-%08X:", username, uid, nthash, lct.Unix())
	return res, nil
}

func SMBPasswdForCurrentUser(plainPassword string) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return "", err
	}
	return SMBPasswd(u.Username, uid, plainPassword, time.Now())
}
