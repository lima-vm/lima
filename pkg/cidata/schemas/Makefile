
CLOUD_INIT_VERSION = 24.1.3

all: versions.schema.cloud-config.json schema-cloud-config-v1.json

%.json:
	curl -fsSLO https://raw.githubusercontent.com/canonical/cloud-init/$(CLOUD_INIT_VERSION)/cloudinit/config/schemas/$@
