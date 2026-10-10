// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

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

func TestNATTable(t *testing.T) {
	t.Run("host loopback reachable by default", func(t *testing.T) {
		assert.DeepEqual(t, natTable("192.168.5.2", false), map[string]string{"192.168.5.2": "127.0.0.1"})
	})

	t.Run("host loopback disabled", func(t *testing.T) {
		assert.DeepEqual(t, natTable("192.168.5.2", true), map[string]string{"192.168.5.2": "0.0.0.1", "0.0.0.0": "0.0.0.1"})
	})
}

func TestNetstackConfigurationNAT(t *testing.T) {
	for _, disable := range []bool{false, true} {
		config, err := netstackConfiguration(&GVisorNetstackOpts{Subnet: "192.168.5.0/24", DisableHostLoopback: disable})
		assert.NilError(t, err)
		assert.DeepEqual(t, config.NAT, natTable("192.168.5.2", disable))
	}
}

func createResolveFile(t *testing.T, file, content string) {
	f, err := os.Create(file)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	writer := bufio.NewWriter(f)
	_, err = writer.WriteString(content)
	assert.NilError(t, err)
	err = writer.Flush()
	assert.NilError(t, err)
}
