package cidata

import (
	"fmt"
	"testing"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/ptr"

	"github.com/lima-vm/lima/pkg/limayaml"
)

func FuzzSetupEnv(f *testing.F) {
	f.Fuzz(func(_ *testing.T, suffix string, localhost bool) {
		var prefix string
		if localhost {
			prefix = "http://localhost:8080/"
		} else {
			prefix = "http://127.0.0.1:8080/"
		}
		envKey := "http_proxy"
		envValue := fmt.Sprintf("%s%s", prefix, suffix)
		templateArgs := TemplateArgs{SlirpGateway: networks.SlirpGateway}
		envAttr := map[string]string{envKey: envValue}
		_, _ = setupEnv(&limayaml.LimaYAML{PropagateProxyEnv: ptr.Of(false), Env: envAttr}, templateArgs)
	})
}
