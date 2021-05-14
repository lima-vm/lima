package limayaml

import (
	_ "embed"
)

//go:embed default.TEMPLATE.yaml
var DefaultTemplate []byte
