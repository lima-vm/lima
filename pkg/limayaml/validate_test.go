// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"fmt"
	"math"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
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
	assert.Error(t, err, "field `provision[0].mode` must one of \"system\", \"user\", \"boot\", \"data\", \"dependency\", \"ansible\", or \"yq\"\n"+
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

func TestValidateProvisionYQ(t *testing.T) {
	images := `images: [{location: /}]`
	param := `param: {"cdi": "true"}`
	// Valid
	validYQProvision := `provision: [{mode: yq, expression: ".features.cdi={{.Param.cdi}}", path: /tmp}]`
	y, err := Load(t.Context(), []byte(param+"\n"+validYQProvision+"\n"+images), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.NilError(t, err)

	// Missing path
	invalidYQProvision := `provision: [{mode: yq, expression: ".features.cdi={{.Param.cdi}}"}]`
	y, err = Load(t.Context(), []byte(param+"\n"+invalidYQProvision+"\n"+images), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.ErrorContains(t, err, "field `provision[0].path` must not be empty when mode is \"yq\"")

	// non-absolute path
	invalidYQProvision = `provision: [{mode: yq, expression: ".features.cdi={{.Param.cdi}}", path: tmp}]`
	y, err = Load(t.Context(), []byte(param+"\n"+invalidYQProvision+"\n"+images), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.ErrorContains(t, err, "field `provision[0].path` must be an absolute path")

	// Missing expression
	invalidYQProvision = `provision: [{mode: yq, path: "/{{.Param.cdi}}"}]`
	y, err = Load(t.Context(), []byte(param+"\n"+invalidYQProvision+"\n"+images), "lima.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.ErrorContains(t, err, "field `provision[0].expression` must not be empty when mode is \"yq\"")

	// Invalid permissions
	invalidYQProvision = `provision: [{mode: yq, expression: ".features.cdi={{.Param.cdi}}", path: /tmp, permissions: 9}]`
	y, err = Load(t.Context(), []byte(param+"\n"+invalidYQProvision+"\n"+images), "lima.yaml")
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

	assert.Error(t, err, "field `os` must be one of [\"Linux\" \"Darwin\" \"FreeBSD\"]; got \"windows\"\n"+
		"field `arch` must be one of [x86_64 aarch64 armv7l ppc64le riscv64 s390x]; got \"unsupported_arch\"\n"+
		"field `images` must be set\n"+
		"field `provision[0].mode` must one of \"system\", \"user\", \"boot\", \"data\", \"dependency\", \"ansible\", or \"yq\"\n"+
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
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "Valid disk size increased",
			yNew:    `disk: 200GiB`,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in both YAMLs",
			yNew:    ``,
			yLatest: ``,
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in new YAMLs",
			yNew:    ``,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "No disk field in latest YAMLs",
			yNew:    `disk: 100GiB`,
			yLatest: ``,
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver", limatype.DefaultDriver()),
		},
		{
			name:    "Disk size shrunk",
			yNew:    `disk: 50GiB`,
			yLatest: `disk: 100GiB`,
			wantErr: fmt.Sprintf("failed to resolve vm for \"\": vmType %q is not a registered driver\n", limatype.DefaultDriver()) +
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

func TestValidate_BalloonVZOnly(t *testing.T) {
	// MemoryBalloon should be rejected when vmType is not "vz".
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.QEMU)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon.Enabled = ptr.Of(true)
	vzOpts.MemoryBalloon.Min = ptr.Of("3GiB")
	vzOpts.MemoryBalloon.IdleTarget = ptr.Of("4GiB")
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "field `vmOpts.vz.memoryBalloon` requires vmType \"vz\"")
}

func TestValidate_BalloonThresholds(t *testing.T) {
	images := `images: [{"location": "/"}]`

	tests := []struct {
		name    string
		balloon limatype.MemoryBalloon
		wantErr string
	}{
		{
			name: "valid balloon config",
			balloon: limatype.MemoryBalloon{
				Enabled:               ptr.Of(true),
				Min:                   ptr.Of("3GiB"),
				IdleTarget:            ptr.Of("4GiB"),
				GrowStepPercent:       ptr.Of(25),
				ShrinkStepPercent:     ptr.Of(10),
				HighPressureThreshold: ptr.Of(0.88),
				LowPressureThreshold:  ptr.Of(0.35),
				Cooldown:              ptr.Of("30s"),
				IdleGracePeriod:       ptr.Of("5m"),
			},
			wantErr: "",
		},
		{
			name: "high threshold less than low threshold",
			balloon: limatype.MemoryBalloon{
				Enabled:               ptr.Of(true),
				Min:                   ptr.Of("3GiB"),
				IdleTarget:            ptr.Of("4GiB"),
				HighPressureThreshold: ptr.Of(0.30),
				LowPressureThreshold:  ptr.Of(0.80),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.highPressureThreshold` (0.30) must be greater than `lowPressureThreshold` (0.80)",
		},
		{
			name: "threshold out of range",
			balloon: limatype.MemoryBalloon{
				Enabled:               ptr.Of(true),
				Min:                   ptr.Of("3GiB"),
				IdleTarget:            ptr.Of("4GiB"),
				HighPressureThreshold: ptr.Of(1.5),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.highPressureThreshold` must be between 0.0 and 1.0",
		},
		{
			name: "NaN high pressure threshold",
			balloon: limatype.MemoryBalloon{
				Enabled:               ptr.Of(true),
				Min:                   ptr.Of("3GiB"),
				IdleTarget:            ptr.Of("4GiB"),
				HighPressureThreshold: ptr.Of(math.NaN()),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.highPressureThreshold` must be between 0.0 and 1.0",
		},
		{
			name: "NaN low pressure threshold",
			balloon: limatype.MemoryBalloon{
				Enabled:              ptr.Of(true),
				Min:                  ptr.Of("3GiB"),
				IdleTarget:           ptr.Of("4GiB"),
				LowPressureThreshold: ptr.Of(math.NaN()),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.lowPressureThreshold` must be between 0.0 and 1.0",
		},
		{
			name: "step percent out of range",
			balloon: limatype.MemoryBalloon{
				Enabled:           ptr.Of(true),
				Min:               ptr.Of("3GiB"),
				IdleTarget:        ptr.Of("4GiB"),
				GrowStepPercent:   ptr.Of(150),
				ShrinkStepPercent: ptr.Of(0),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.growStepPercent` must be between 1 and 100",
		},
		{
			name: "invalid cooldown duration",
			balloon: limatype.MemoryBalloon{
				Enabled:    ptr.Of(true),
				Min:        ptr.Of("3GiB"),
				IdleTarget: ptr.Of("4GiB"),
				Cooldown:   ptr.Of("not-a-duration"),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.cooldown` must be a valid duration",
		},
		{
			name: "min greater than idleTarget",
			balloon: limatype.MemoryBalloon{
				Enabled:    ptr.Of(true),
				Min:        ptr.Of("8GiB"),
				IdleTarget: ptr.Of("4GiB"),
			},
			wantErr: "field `vmOpts.vz.memoryBalloon.min` must be less than `idleTarget`",
		},
		{
			name: "disabled balloon skips validation",
			balloon: limatype.MemoryBalloon{
				Enabled:               ptr.Of(false),
				HighPressureThreshold: ptr.Of(1.5), // would be invalid if enabled
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(images), "lima.yaml")
			assert.NilError(t, err)
			y.VMType = ptr.Of(limatype.VZ)
			y.Memory = ptr.Of("12GiB")

			var vzOpts limatype.VZOpts
			vzOpts.MemoryBalloon = tt.balloon
			var opts any
			_ = Convert(vzOpts, &opts, "")
			y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

			err = Validate(y, false)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_AutoPauseVZOnly(t *testing.T) {
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.QEMU) // Not VZ.

	var vzOpts limatype.VZOpts
	vzOpts.AutoPause = limatype.AutoPause{
		Enabled:     ptr.Of(true),
		IdleTimeout: ptr.Of("15m"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "requires vmType")
}

func TestValidate_AutoPauseRules(t *testing.T) {
	tests := []struct {
		name    string
		ap      limatype.AutoPause
		balloon limatype.MemoryBalloon
		wantErr string
	}{
		{
			name:    "valid config",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("15m"), ResumeTimeout: ptr.Of("30s")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")},
			wantErr: "",
		},
		{
			name:    "disabled skips validation",
			ap:      limatype.AutoPause{Enabled: ptr.Of(false), IdleTimeout: ptr.Of("bad")},
			balloon: limatype.MemoryBalloon{},
			wantErr: "",
		},
		{
			name:    "invalid idleTimeout",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("notaduration")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")},
			wantErr: "idleTimeout",
		},
		{
			name:    "idleTimeout too short",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("30s"), ResumeTimeout: ptr.Of("30s")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")},
			wantErr: "at least 1m",
		},
		{
			name:    "invalid resumeTimeout",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("15m"), ResumeTimeout: ptr.Of("bad")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")},
			wantErr: "resumeTimeout",
		},
		{
			name:    "resumeTimeout too short",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("15m"), ResumeTimeout: ptr.Of("2s")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")},
			wantErr: "at least 5s",
		},
		{
			name:    "requires balloon enabled",
			ap:      limatype.AutoPause{Enabled: ptr.Of(true), IdleTimeout: ptr.Of("15m"), ResumeTimeout: ptr.Of("30s")},
			balloon: limatype.MemoryBalloon{Enabled: ptr.Of(false)},
			wantErr: "memoryBalloon.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
			assert.NilError(t, err)
			y.VMType = ptr.Of(limatype.VZ)
			y.Memory = ptr.Of("12GiB")

			var vzOpts limatype.VZOpts
			vzOpts.AutoPause = tt.ap
			vzOpts.MemoryBalloon = tt.balloon
			var opts any
			_ = Convert(vzOpts, &opts, "")
			y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

			err = Validate(y, false)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

// --- Validation edge case tests ---

func TestValidate_BalloonWithNilMemory(t *testing.T) {
	// Balloon enabled but Memory is nil — should not panic.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = nil // No memory set.

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("2GiB"),
		IdleTarget: ptr.Of("4GiB"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	// Should not panic; validation may succeed or fail but not crash.
	_ = Validate(y, false)
}

func TestValidate_BalloonMinEqualsIdleTarget(t *testing.T) {
	// min == idleTarget should fail (must be less than).
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("4GiB"),
		IdleTarget: ptr.Of("4GiB"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "must be less than")
}

func TestValidate_BalloonEmptyDuration(t *testing.T) {
	// Empty string for cooldown should fail.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("2GiB"),
		IdleTarget: ptr.Of("4GiB"),
		Cooldown:   ptr.Of(""),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "cooldown")
}

func TestValidate_AutoPauseNegativeDuration(t *testing.T) {
	// Negative idleTimeout should fail.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.AutoPause = limatype.AutoPause{
		Enabled:     ptr.Of(true),
		IdleTimeout: ptr.Of("-5m"),
	}
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "at least 1m")
}

func TestValidate_AutoPauseEmptyDuration(t *testing.T) {
	// Empty resumeTimeout string should fail.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.AutoPause = limatype.AutoPause{
		Enabled:       ptr.Of(true),
		IdleTimeout:   ptr.Of("15m"),
		ResumeTimeout: ptr.Of(""),
	}
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "resumeTimeout")
}

func TestValidate_NilVMOptsDoesNotPanic(t *testing.T) {
	// VMOpts is nil — validation should not panic.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.VMOpts = nil

	// Should not panic.
	_ = Validate(y, false)
}

func TestValidate_BalloonIdleTargetExceedsMemory(t *testing.T) {
	// idleTarget > Memory should fail.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("8GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("2GiB"),
		IdleTarget: ptr.Of("16GiB"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "must not exceed")
}

func TestValidate_BalloonIdleTargetEqualsMemory(t *testing.T) {
	// idleTarget == Memory should pass.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("8GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("2GiB"),
		IdleTarget: ptr.Of("8GiB"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.NilError(t, err)
}

func TestValidate_BalloonStepBoundaries(t *testing.T) {
	// Exact boundary values: 1 and 100 should pass.
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:           ptr.Of(true),
		Min:               ptr.Of("2GiB"),
		IdleTarget:        ptr.Of("4GiB"),
		GrowStepPercent:   ptr.Of(1),
		ShrinkStepPercent: ptr.Of(100),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.NilError(t, err)
}

func TestValidate_BalloonThresholdBoundaries(t *testing.T) {
	// Exact boundary values: 0.0 and 1.0 should pass (when high > low).
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:               ptr.Of(true),
		Min:                   ptr.Of("2GiB"),
		IdleTarget:            ptr.Of("4GiB"),
		HighPressureThreshold: ptr.Of(1.0),
		LowPressureThreshold:  ptr.Of(0.0),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.NilError(t, err)
}

func TestValidate_BalloonThresholdEquality(t *testing.T) {
	// high == low should fail (must be strictly greater).
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:               ptr.Of(true),
		Min:                   ptr.Of("2GiB"),
		IdleTarget:            ptr.Of("4GiB"),
		HighPressureThreshold: ptr.Of(0.5),
		LowPressureThreshold:  ptr.Of(0.5),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "greater than")
}

func TestValidate_BalloonMinZero(t *testing.T) {
	// min = "0" should fail (must be > 0).
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{
		Enabled:    ptr.Of(true),
		Min:        ptr.Of("0"),
		IdleTarget: ptr.Of("4GiB"),
	}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "greater than 0")
}

func TestValidate_AutoPauseExactBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		idleTimeout   string
		resumeTimeout string
		wantErr       string
	}{
		{"exact 1m idle passes", "1m", "5s", ""},
		{"exact 5s resume passes", "5m", "5s", ""},
		{"59s idle fails", "59s", "5s", "at least 1m"},
		{"4s resume fails", "5m", "4s", "at least 5s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
			assert.NilError(t, err)
			y.VMType = ptr.Of(limatype.VZ)
			y.Memory = ptr.Of("12GiB")

			var vzOpts limatype.VZOpts
			vzOpts.AutoPause = limatype.AutoPause{
				Enabled:       ptr.Of(true),
				IdleTimeout:   ptr.Of(tt.idleTimeout),
				ResumeTimeout: ptr.Of(tt.resumeTimeout),
			}
			vzOpts.MemoryBalloon = limatype.MemoryBalloon{Enabled: ptr.Of(true), Min: ptr.Of("2GiB"), IdleTarget: ptr.Of("4GiB")}
			var opts any
			_ = Convert(vzOpts, &opts, "")
			y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

			err = Validate(y, false)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidate_AutoPauseNoBalloonInVMOpts(t *testing.T) {
	// autoPause enabled but memoryBalloon not present (defaults to disabled).
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.AutoPause = limatype.AutoPause{
		Enabled:       ptr.Of(true),
		IdleTimeout:   ptr.Of("15m"),
		ResumeTimeout: ptr.Of("30s"),
	}
	// No MemoryBalloon set at all.
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}

	err = Validate(y, false)
	assert.ErrorContains(t, err, "memoryBalloon.enabled")
}

// --- Phase 7: IdleSignals Validation Tests ---

// makeAutoPauseYAML builds a LimaYAML with auto-pause enabled and balloon enabled.
func makeAutoPauseYAML(t *testing.T, ap limatype.AutoPause) *limatype.LimaYAML {
	t.Helper()
	y, err := Load(t.Context(), []byte(`images: [{"location": "/"}]`), "lima.yaml")
	assert.NilError(t, err)
	y.VMType = ptr.Of(limatype.VZ)
	y.Memory = ptr.Of("12GiB")

	var vzOpts limatype.VZOpts
	vzOpts.AutoPause = ap
	vzOpts.MemoryBalloon = limatype.MemoryBalloon{Enabled: ptr.Of(true)}
	var opts any
	_ = Convert(vzOpts, &opts, "")
	y.VMOpts = limatype.VMOpts{limatype.VZ: opts}
	return y
}

func TestValidate_AutoPauseCPUThresholdRange(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		wantErr   bool
	}{
		{"negative", -1.0, true},
		{"too high", 101.0, true},
		{"NaN", math.NaN(), true},
		{"zero", 0.0, false},
		{"max", 100.0, false},
		{"mid", 50.0, false},
		{"default", 0.5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ap := limatype.AutoPause{
				Enabled:       ptr.Of(true),
				IdleTimeout:   ptr.Of("15m"),
				ResumeTimeout: ptr.Of("30s"),
				IdleSignals: limatype.IdleSignals{
					ContainerCPUThreshold: ptr.Of(tt.threshold),
				},
			}
			y := makeAutoPauseYAML(t, ap)
			err := Validate(y, false)
			if tt.wantErr {
				assert.ErrorContains(t, err, "containerCPUThreshold")
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestValidate_AutoPauseCPUDisabledWithThreshold(t *testing.T) {
	// containerCPU disabled but threshold set → warning only, no error.
	ap := limatype.AutoPause{
		Enabled:       ptr.Of(true),
		IdleTimeout:   ptr.Of("15m"),
		ResumeTimeout: ptr.Of("30s"),
		IdleSignals: limatype.IdleSignals{
			ContainerCPU:          ptr.Of(false),
			ContainerCPUThreshold: ptr.Of(5.0),
		},
	}
	y := makeAutoPauseYAML(t, ap)
	err := Validate(y, false)
	assert.NilError(t, err, "disabled CPU with threshold should warn but not error")
}

func TestValidate_AutoPauseIdleSignalsZeroValue(t *testing.T) {
	// Zero-value IdleSignals (all nil) should pass validation — no threshold set.
	ap := limatype.AutoPause{
		Enabled:       ptr.Of(true),
		IdleTimeout:   ptr.Of("15m"),
		ResumeTimeout: ptr.Of("30s"),
	}
	y := makeAutoPauseYAML(t, ap)
	err := Validate(y, false)
	assert.NilError(t, err)
}
