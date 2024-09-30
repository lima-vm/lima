package cidata

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/lima-vm/lima/pkg/networks"

	"gotest.tools/v3/assert"
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
