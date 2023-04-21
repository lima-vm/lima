package usernet

import (
	"bufio"
	"os"
	"path"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSearchDomain(t *testing.T) {

	if runtime.GOOS == "windows" {
		// FIXME: `TempDir RemoveAll cleanup: remove C:\users\runner\Temp\TestDownloadLocalwithout_digest2738386858\002\test-file: Sharing violation.`
		t.Skip("Skipping on windows")
	}

	t.Run("search domain", func(t *testing.T) {
		resolvFile := path.Join(t.TempDir(), "resolv.conf")
		createResolveFile(t, resolvFile, `
search test.com lima.net
nameserver 192.168.0.100
nameserver 8.8.8.8`)

		dns := resolveSearchDomain(resolvFile)
		assert.DeepEqual(t, dns, []string{"test.com", "lima.net"})
	})

	t.Run("empty search domain", func(t *testing.T) {
		resolvFile := path.Join(t.TempDir(), "resolv.conf")
		createResolveFile(t, resolvFile, `
nameserver 192.168.0.100
nameserver 8.8.8.8`)

		dns := resolveSearchDomain(resolvFile)
		var expected []string
		assert.DeepEqual(t, dns, expected)
	})
}

func createResolveFile(t *testing.T, file string, content string) {
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(f)
	_, err = writer.Write([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Flush()
	if err != nil {
		t.Fatal(err)
	}
}
