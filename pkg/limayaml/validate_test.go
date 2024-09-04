package limayaml

import (
	"os"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateEmpty(t *testing.T) {
	y, err := Load([]byte{}, "empty.yaml")
	assert.NilError(t, err)
	err = Validate(y, false)
	assert.Error(t, err, "field `images` must be set")
}

// Note: can't embed symbolic links, use "os"

func TestValidateDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		// FIXME: `assertion failed: error is not nil: field `mounts[1].location` must be an absolute path, got "/tmp/lima"`
		t.Skip("Skipping on windows")
	}

	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	y, err := Load(bytes, "default.yaml")
	assert.NilError(t, err)
	err = Validate(y, true)
	assert.NilError(t, err)
}

func TestValidateParamIsUsed(t *testing.T) {
	paramYaml := `param:
  name: value`
	_, err := Load([]byte(paramYaml), "paramIsNotUsed.yaml")
	assert.Error(t, err, "field `param` key \"name\" is not used in any provision, probe, copyToHost, or portForward")

	fieldsUsingParam := []string{
		`mounts: [{"location": "/tmp/{{ .Param.name }}"}]`,
		`mounts: [{"location": "/tmp", mountPoint: "/tmp/{{ .Param.name }}"}]`,
		`provision: [{"script": "echo {{ .Param.name }}"}]`,
		`probes: [{"script": "echo {{ .Param.name }}"}]`,
		`copyToHost: [{"guest": "/tmp/{{ .Param.name }}", "host": "/tmp"}]`,
		`copyToHost: [{"guest": "/tmp", "host": "/tmp/{{ .Param.name }}"}]`,
		`portForwards: [{"guestSocket": "/tmp/{{ .Param.name }}", "hostSocket": "/tmp"}]`,
		`portForwards: [{"guestSocket": "/tmp", "hostSocket": "/tmp/{{ .Param.name }}"}]`,
	}
	for _, fieldUsingParam := range fieldsUsingParam {
		_, err = Load([]byte(fieldUsingParam+"\n"+paramYaml), "paramIsUsed.yaml")
		//
		assert.NilError(t, err)
	}

	// use "{{if .Param.rootful \"true\"}}{{else}}{{end}}"" in provision, probe, copyToHost, and portForward
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
		`portForwards: [{"guestSocket": "/var/run/docker.sock", "hostSocket": "{{.Dir}}/sock/docker-{{if eq .Param.rootful \"true\"}}rootfule{{else}}rootless{{end}}.sock"}]`,
	}
	for _, fieldUsingIfParamRootfulTrue := range fieldsUsingIfParamRootfulTrue {
		_, err = Load([]byte(fieldUsingIfParamRootfulTrue+"\n"+rootfulYaml), "paramIsUsed.yaml")
		//
		assert.NilError(t, err)
	}

	// use rootFul instead of rootful
	rootFulYaml := `param:
  rootFul: true`
	for _, fieldUsingIfParamRootfulTrue := range fieldsUsingIfParamRootfulTrue {
		_, err = Load([]byte(fieldUsingIfParamRootfulTrue+"\n"+rootFulYaml), "paramIsUsed.yaml")
		//
		assert.Error(t, err, "field `param` key \"rootFul\" is not used in any provision, probe, copyToHost, or portForward")
	}
}
