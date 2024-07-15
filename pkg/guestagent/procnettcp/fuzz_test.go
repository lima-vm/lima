package procnettcp

import (
	"bytes"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Fuzz(func(_ *testing.T, data []byte, tcp6 bool) {
		var kind Kind
		if tcp6 {
			kind = TCP6
		} else {
			kind = TCP
		}
		_, _ = Parse(bytes.NewReader(data), kind)
	})
}
