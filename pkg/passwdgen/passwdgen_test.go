package passwdgen

import "testing"

func TestGeneratePassword(t *testing.T) {
	for i := 0; i < 10; i++ {
		s := GeneratePassword(32)
		t.Logf("generated password %d: length %d: %q", i, len(s), s)
	}
}
