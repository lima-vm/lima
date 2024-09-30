package cidata

import (
	"testing"

	"github.com/lima-vm/lima/pkg/networks"
)

func FuzzSetupEnv(f *testing.F) {
	f.Fuzz(func(_ *testing.T, suffix string, localhost bool) {
		prefix := "http://127.0.0.1:8080/"
		if localhost {
			prefix = "http://localhost:8080/"
		}
		_, _ = setupEnv(map[string]string{"http_proxy": prefix + suffix}, false, networks.SlirpGateway)
	})
}
