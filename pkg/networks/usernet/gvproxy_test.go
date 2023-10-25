package usernet

import (
	"bufio"
	"os"
	"path"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSearchDomain(t *testing.T) {
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
	t.Cleanup(func() { _ = f.Close() })
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
