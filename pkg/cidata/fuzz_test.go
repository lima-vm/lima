package cidata

import (
	"testing"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/ptr"
)

func FuzzSetupEnv(f *testing.F) {
	f.Fuzz(func(_ *testing.T, suffix string, localhost bool) {
		prefix := "http://127.0.0.1:8080/"
		if localhost {
			prefix = "http://localhost:8080/"
		}
		templateArgs := TemplateArgs{SlirpGateway: networks.SlirpGateway}
		_, _ = setupEnv(&limayaml.LimaYAML{
			PropagateProxyEnv: ptr.Of(false),
			Env:               map[string]string{"http_proxy": prefix + suffix},
		}, templateArgs)
	})
}
