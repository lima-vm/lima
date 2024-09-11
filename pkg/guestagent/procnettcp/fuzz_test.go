package procnettcp

import (
	"bytes"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, tcp6 bool) {
		kind := TCP
		if tcp6 {
			kind = TCP6
		}
		_, _ = Parse(bytes.NewReader(data), kind)
	})
}
