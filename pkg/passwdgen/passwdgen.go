package passwdgen

import (
	"crypto/rand"
	"encoding/base64"
)

func GeneratePassword(strLen int) string {
	bLen := strLen * 2
	b := make([]byte, bLen)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)[:strLen]
}
