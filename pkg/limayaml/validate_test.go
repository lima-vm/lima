// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/version"
)

func TestValidateEmpty(t *testing.T) {
	y, err := Load(t.Context(), []byte{}, "empty.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.Error(t, err, "field `images` must be set")
}

func TestValidateMinimumLimaVersion(t *testing.T) {
	images := `images: [{"location": "/"}]`

	tests := []struct {
		name               string
		currentVersion     string
		minimumLimaVersion string
		wantErr            string
	}{
		{
			name:               "minimumLimaVersion less than current version",
			currentVersion:     "1.1.1-114-g5bf5e513",
			minimumLimaVersion: "1.1.0",
			wantErr:            "",
		},
		{
			name:               "minimumLimaVersion greater than current version",
			currentVersion:     "1.1.1-114-g5bf5e513",
			minimumLimaVersion: "1.1.2",
			wantErr:            `template requires Lima version "1.1.2"; this is only "1.1.1-114-g5bf5e513"`,
		},
		{
			name:               "invalid current version",
			currentVersion:     "<unknown>",
			minimumLimaVersion: "0.8.0",
			wantErr:            "", // Unparsable versions are treated as "latest"
		},
		{
			name:               "invalid minimumLimaVersion",
			currentVersion:     "1.1.1-114-g5bf5e513",
			minimumLimaVersion: "invalid",
			wantErr:            "field `minimumLimaVersion` must be a semvar value, got \"invalid\": invalid is not in dotted-tri format", // Only parse error, no comparison error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldVersion := version.Version
			version.Version = tt.currentVersion
			t.Cleanup(func() { version.Version = oldVersion })

			y, err := Load(t.Context(), []byte("minimumLimaVersion: "+tt.minimumLimaVersion+"\n"+images), "lima.yaml")
			assert.NilError(t, err)

			err = Validate(y, false)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDigest(t *testing.T) {
	images := `images: [{"location": "https://cloud-images.ubuntu.com/releases/oracular/release-20250701/ubuntu-24.10-server-cloudimg-amd64.img",digest: "69f31d3208895e5f646e345fbc95190e5e311ecd1359a4d6ee2c0b6483ceca03"}]`
	validProbe := `probes: [{"script": "#!foo"}]`
	y, err := Load(t.Context(), []byte(validProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.Error(t, err, "field `images[0].digest` is invalid: 69f31d3208895e5f646e345fbc95190e5e311ecd1359a4d6ee2c0b6483ceca03: invalid checksum digest format")

	images2 := `images: [{"location": "https://cloud-images.ubuntu.com/releases/oracular/release-20250701/ubuntu-24.10-server-cloudimg-amd64.img",digest: "sha001:69f31d3208895e5f646e345fbc95190e5e311ecd1359a4d6ee2c0b6483ceca03"}]`
	y2, err := Load(t.Context(), []byte(validProbe+"\n"+images2), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y2, false)
	assert.Error(t, err, "field `images[0].digest` is invalid: sha001:69f31d3208895e5f646e345fbc95190e5e311ecd1359a4d6ee2c0b6483ceca03: unsupported digest algorithm")
}

func TestValidateProbes(t *testing.T) {
	images := `images: [{"location": "/"}]`
	validProbe := `probes: [{"script": "#!foo"}]`
	y, err := Load(t.Context(), []byte(validProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	invalidProbe := `probes: [{"script": "foo"}]`
	y, err = Load(t.Context(), []byte(invalidProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `probe[0].script` must start with a '#!' line")

	invalidProbe = `probes: [{file: {digest: decafbad}}]`
	y, err = Load(t.Context(), []byte(invalidProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `probe[0].file.digest` support is not yet implemented\n"+
		"field `probe[0].script` must start with a '#!' line")
}

func TestValidateProvisionMode(t *testing.T) {
	images := `images: [{location: /}]`
	provisionBoot := `provision: [{mode: boot, script: "touch /tmp/param-$PARAM_BOOT"}]`
	y, err := Load(t.Context(), []byte(provisionBoot+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	provisionUser := `provision: [{mode: user, script: "touch /tmp/param-$PARAM_USER"}]`
	y, err = Load(t.Context(), []byte(provisionUser+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	provisionDependency := `provision: [{mode: ansible, script: "touch /tmp/param-$PARAM_DEPENDENCY"}]`
	y, err = Load(t.Context(), []byte(provisionDependency+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	provisionInvalid := `provision: [{mode: invalid}]`
	y, err = Load(t.Context(), []byte(provisionInvalid+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `provision[0].mode` must one of \"system\", \"user\", \"boot\", \"data\", \"dependency\", or \"ansible\"\n"+
		"field `provision[0].script` must not be empty")
}

func TestValidateProvisionData(t *testing.T) {
	images := `images: [{location: /}]`
	validData := `provision: [{mode: data, path: /tmp, content: hello}]`
	y, err := Load(t.Context(), []byte(validData+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	invalidData := `provision: [{mode: data, content: hello}]`
	y, err = Load(t.Context(), []byte(invalidData+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `provision[0].path` must not be empty when mode is \"data\"")

	invalidData = `provision: [{mode: data, path: /tmp, content: hello, permissions: 9}]`
	y, err = Load(t.Context(), []byte(invalidData+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.ErrorContains(t, err, "provision[0].permissions` must be an octal number")
}

func TestValidateAdditionalDisks(t *testing.T) {
	images := `images: [{"location": "/"}]`

	validDisks := `
additionalDisks:
  - name: "disk1"
  - name: "disk2"
`
	y, err := Load(t.Context(), []byte(validDisks+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	invalidDisks := `
additionalDisks:
  - name: ""
`
	y, err = Load(t.Context(), []byte(invalidDisks+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `additionalDisks[0].name is invalid`: identifier must not be empty")
}

func TestValidateParamName(t *testing.T) {
	images := `images: [{"location": "/"}]`
	validProvision := `provision: [{"script": "echo $PARAM_name $PARAM_NAME $PARAM_Name_123"}]`
	validParam := []string{
		`param: {"name": "value"}`,
		`param: {"NAME": "value"}`,
		`param: {"Name_123": "value"}`,
	}
	for _, param := range validParam {
		y, err := Load(t.Context(), []byte(param+"\n"+validProvision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.NilError(t, err)
	}

	invalidProvision := `provision: [{"script": "echo $PARAM__Name $PARAM_3Name $PARAM_Last.Name"}]`
	invalidParam := []string{
		`param: {"_Name": "value"}`,
		`param: {"3Name": "value"}`,
		`param: {"Last.Name": "value"}`,
	}
	for _, param := range invalidParam {
		y, err := Load(t.Context(), []byte(param+"\n"+invalidProvision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.ErrorContains(t, err, "name does not match regex")
	}
}

func TestValidateParamValue(t *testing.T) {
	images := `images: [{"location": "/"}]`
	provision := `provision: [{"script": "echo $PARAM_name"}]`
	validParam := []string{
		`param: {"name": ""}`,
		`param: {"name": "foo bar"}`,
		`param: {"name": "foo\tbar"}`,
		`param: {"name": "Symbols ½ and emoji → 👀"}`,
	}
	for _, param := range validParam {
		y, err := Load(t.Context(), []byte(param+"\n"+provision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.NilError(t, err)
	}

	invalidParam := []string{
		`param: {"name": "The end.\n"}`,
		`param: {"name": "\r"}`,
	}
	for _, param := range invalidParam {
		y, err := Load(t.Context(), []byte(param+"\n"+provision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.ErrorContains(t, err, "value contains unprintable character")
	}
}

func TestValidateParamIsUsed(t *testing.T) {
	paramYaml := `param:
  name: value`
	_, err := Load(t.Context(), []byte(paramYaml), "paramIsNotUsed.yaml")
	assert.Error(t, err, "field `param` key \"name\" is not used in any provision, probe, copyToHost, or portForward")

	fieldsUsingParam := []string{
		`mounts: [{"location": "/tmp/{{ .Param.name }}"}]`,
		`mounts: [{"location": "/tmp", mountPoint: "/tmp/{{ .Param.name }}"}]`,
		`provision: [{"script": "echo {{ .Param.name }}"}]`,
		`provision: [{"script": "echo $PARAM_name"}]`,
		`probes: [{"script": "echo {{ .Param.name }}"}]`,
		`probes: [{"script": "echo $PARAM_name"}]`,
		`copyToHost: [{"guest": "/tmp/{{ .Param.name }}", "host": "/tmp"}]`,
		`copyToHost: [{"guest": "/tmp", "host": "/tmp/{{ .Param.name }}"}]`,
		`portForwards: [{"guestSocket": "/tmp/{{ .Param.name }}", "hostSocket": "/tmp"}]`,
		`portForwards: [{"guestSocket": "/tmp", "hostSocket": "/tmp/{{ .Param.name }}"}]`,
	}
	for _, fieldUsingParam := range fieldsUsingParam {
		_, err = Load(t.Context(), []byte(fieldUsingParam+"\n"+paramYaml), "paramIsUsed.yaml")
		//
		assert.NilError(t, err)
	}

	// use "{{if eq .Param.rootful \"true\"}}…{{else}}…{{end}}" in provision, probe, copyToHost, and portForward
	rootfulYaml := `param:
  rootful: true`
	fieldsUsingIfParamRootfulTrue := []string{
		`mounts: [{"location": "/tmp/{{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}", "mountPoint": "/tmp"}]`,
		`mounts: [{"location": "/tmp", "mountPoint": "/tmp/{{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}"}]`,
		`provision: [{"script": "echo {{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}"}]`,
		`probes: [{"script": "echo {{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}"}]`,
		`copyToHost: [{"guest": "/tmp/{{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}", "host": "/tmp"}]`,
		`copyToHost: [{"guest": "/tmp", "host": "/tmp/{{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}"}]`,
		`portForwards: [{"guestSocket": "{{if eq .Param.rootful \"true\"}}/var/run{{else}}/run/user/{{.UID}}{{end}}/docker.sock", "hostSocket": "{{.Dir}}/sock/docker.sock"}]`,
		`portForwards: [{"guestSocket": "/var/run/docker.sock", "hostSocket": "{{.Dir}}/sock/docker-{{if eq .Param.rootful \"true\"}}rootful{{else}}rootless{{end}}.sock"}]`,
	}
	for _, fieldUsingIfParamRootfulTrue := range fieldsUsingIfParamRootfulTrue {
		_, err = Load(t.Context(), []byte(fieldUsingIfParamRootfulTrue+"\n"+rootfulYaml), "paramIsUsed.yaml")
		//
		assert.NilError(t, err)
	}

	// use rootFul instead of rootful
	rootFulYaml := `param:
  rootFul: true`
	for _, fieldUsingIfParamRootfulTrue := range fieldsUsingIfParamRootfulTrue {
		_, err = Load(t.Context(), []byte(fieldUsingIfParamRootfulTrue+"\n"+rootFulYaml), "paramIsUsed.yaml")
		//
		assert.Error(t, err, "field `param` key \"rootFul\" is not used in any provision, probe, copyToHost, or portForward")
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	yamlWithMultipleErrors := `
os: windows
arch: unsupported_arch
portForwards:
  - guestPort: 22
    hostPort: 2222
  - guestPort: 8080
    hostPort: 65536
provision:
  - mode: invalid_mode
    script: echo test
  - mode: data
    content: test
`

	y, err := Load(t.Context(), []byte(yamlWithMultipleErrors), "multiple-errors.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	t.Logf("Validation errors: %v", err)

	assert.Error(t, err, "field `os` must be \"Linux\"; got \"windows\"\n"+
		"field `arch` must be one of [x86_64 aarch64 armv7l ppc64le riscv64 s390x]; got \"unsupported_arch\"\n"+
		"field `images` must be set\n"+
		"field `provision[0].mode` must one of \"system\", \"user\", \"boot\", \"data\", \"dependency\", or \"ansible\"\n"+
		"field `provision[1].path` must not be empty when mode is \"data\"")
}

func TestValidateAgainstLatestConfig(t *testing.T) {
	tests := []struct {
		name    string
		yNew    string
		yLatest string
		wantErr string
	}{
		{
			name:    "Valid disk size unchanged",
			yNew:    `disk: 100GiB`,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "Valid disk size increased",
			yNew:    `disk: 200GiB`,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in both YAMLs",
			yNew:    ``,
			yLatest: ``,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in new YAMLs",
			yNew:    ``,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in latest YAMLs",
			yNew:    `disk: 100GiB`,
			yLatest: ``,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "Disk size shrunk",
			yNew:    `disk: 50GiB`,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to accept config for \"\": vmType %q is not a registered driver\n", limatype.DefaultDriver()) +
				"field `disk`: shrinking the disk (100GiB --> 50GiB) is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAgainstLatestConfig(t.Context(), []byte(tt.yNew), []byte(tt.yLatest))
			assert.Error(t, err, tt.wantErr)
		})
	}
}
