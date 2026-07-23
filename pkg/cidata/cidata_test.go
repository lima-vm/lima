// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cidata

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/networks"
)

func fakeLookupIP(_ string) []net.IP {
	return []net.IP{net.IPv4(127, 0, 0, 0)}
}

func TestSetupEnv(t *testing.T) {
	netLookupIP = fakeLookupIP
	urlMustParse := func(urlStr string) *url.URL {
		u, err := url.Parse(urlStr)
		assert.NilError(t, err)
		return u
	}

	httpProxyCases := []*url.URL{
		urlMustParse("http://127.0.0.1"),
		urlMustParse("http://127.0.0.1:8080"),
		urlMustParse("https://127.0.0.1:8080"),
		urlMustParse("sock4://127.0.0.1:8080"),
		urlMustParse("sock5://127.0.0.1:8080"),
		urlMustParse("http://127.0.0.1:8080/"),
		urlMustParse("http://127.0.0.1:8080/path"),
		urlMustParse("http://localhost:8080"),
		urlMustParse("http://localhost:8080/"),
		urlMustParse("http://localhost:8080/path"),
		urlMustParse("http://docker.for.mac.localhost:8080"),
		urlMustParse("http://docker.for.mac.localhost:8080/"),
		urlMustParse("http://docker.for.mac.localhost:8080/path"),
	}

	for _, httpProxy := range httpProxyCases {
		t.Run(httpProxy.Host, func(t *testing.T) {
			envKey := "http_proxy"
			envValue := httpProxy.String()
			envs, err := setupEnv(map[string]string{envKey: envValue}, false, networks.SlirpGateway)
			assert.NilError(t, err)
			assert.Equal(t, envs[envKey], strings.ReplaceAll(envValue, httpProxy.Hostname(), networks.SlirpGateway))
		})
	}
}

func TestSetupInvalidEnv(t *testing.T) {
	envKey := "http_proxy"
	envValue := "://localhost:8080"
	envs, err := setupEnv(map[string]string{envKey: envValue}, false, networks.SlirpGateway)
	assert.NilError(t, err)
	assert.Equal(t, envs[envKey], envValue)
}

func TestAppendProvisionEntries(t *testing.T) {
	scriptBoot := "Write-Host boot"
	scriptSystem := "Write-Host system"
	contentData := "hello"
	exprYQ := ".foo=1"
	provisions := []limatype.Provision{
		{Mode: limatype.ProvisionModeBoot, Script: &scriptBoot},
		{Mode: limatype.ProvisionModeSystem, Script: &scriptSystem},
		{Mode: limatype.ProvisionModeData, ProvisionData: limatype.ProvisionData{Content: &contentData}},
		{Mode: limatype.ProvisionModeYQ, Expression: &exprYQ},
	}

	layoutWithoutBoot, err := appendProvisionEntries(nil, provisions, false)
	assert.NilError(t, err)
	assert.Assert(t, !hasEntryPath(layoutWithoutBoot, "provision.boot/00000000"))
	assert.Assert(t, hasEntryPath(layoutWithoutBoot, "provision.system/00000001"))

	layoutWithBoot, err := appendProvisionEntries(nil, provisions, true)
	assert.NilError(t, err)
	assert.Assert(t, hasEntryPath(layoutWithBoot, "provision.boot/00000000"))
	assert.Assert(t, hasEntryPath(layoutWithBoot, "provision.data/00000002"))
	assert.Assert(t, hasEntryPath(layoutWithBoot, "provision.yq/00000003"))

	gotData, err := readEntry(layoutWithBoot, "provision.data/00000002")
	assert.NilError(t, err)
	assert.Equal(t, gotData, contentData)
}

func hasEntryPath(layout []iso9660util.Entry, path string) bool {
	for _, e := range layout {
		if e.Path == path {
			return true
		}
	}
	return false
}

func readEntry(layout []iso9660util.Entry, path string) (string, error) {
	for _, e := range layout {
		if e.Path != path {
			continue
		}
		b, err := io.ReadAll(e.Reader)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmt.Errorf("entry not found: %s", path)
}
