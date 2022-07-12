package dirnames

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

// Note: can't use t.TempDir(), because it is _always_ long... <sigh>
//       instead use os.TempDir(), something like `C:\users\anders\Temp`

func TestShortPathNameShort(t *testing.T) {
	d := os.TempDir()
	l := filepath.Join(d, "foo")
	err := os.Mkdir(l, 0755)
	assert.NilError(t, err)
	s, err := ShortPathName(l)
	assert.NilError(t, err)
	t.Logf("%s => %s", l, s)
	os.RemoveAll(l)
}

func TestShortPathNameLong(t *testing.T) {
	d := os.TempDir()
	l := filepath.Join(d, "baaaaaaaaaar")
	err := os.Mkdir(l, 0755)
	assert.NilError(t, err)
	s, err := ShortPathName(l)
	assert.NilError(t, err)
	t.Logf("%s => %s", l, s)
	fi, err := os.Stat(s)
	assert.NilError(t, err)
	assert.Assert(t, fi.Mode().IsDir())
	os.RemoveAll(l)
}
