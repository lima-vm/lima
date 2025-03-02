/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

func TestValidateProbes(t *testing.T) {
	images := `images: [{"location": "/"}]`
	validProbe := `probes: [{"script": "#!foo"}]`
	y, err := Load([]byte(validProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	invalidProbe := `probes: [{"script": "foo"}]`
	y, err = Load([]byte(invalidProbe+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `probe[0].script` must start with a '#!' line")
}

func TestValidateAdditionalDisks(t *testing.T) {
	images := `images: [{"location": "/"}]`

	validDisks := `
additionalDisks:
  - name: "disk1"
  - name: "disk2"
`
	y, err := Load([]byte(validDisks+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.NilError(t, err)

	invalidDisks := `
additionalDisks:
  - name: ""
`
	y, err = Load([]byte(invalidDisks+"\n"+images), "lima.yaml")
	assert.NilError(t, err)

	err = Validate(y, false)
	assert.Error(t, err, "field `additionalDisks[0].name is invalid`: identifier must not be empty: invalid argument")
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
		y, err := Load([]byte(param+"\n"+validProvision+"\n"+images), "lima.yaml")
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
		y, err := Load([]byte(param+"\n"+invalidProvision+"\n"+images), "lima.yaml")
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
		y, err := Load([]byte(param+"\n"+provision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.NilError(t, err)
	}

	invalidParam := []string{
		`param: {"name": "The end.\n"}`,
		`param: {"name": "\r"}`,
	}
	for _, param := range invalidParam {
		y, err := Load([]byte(param+"\n"+provision+"\n"+images), "lima.yaml")
		assert.NilError(t, err)

		err = Validate(y, false)
		assert.ErrorContains(t, err, "value contains unprintable character")
	}
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
		`provision: [{"script": "echo $PARAM_name"}]`,
		`probes: [{"script": "echo {{ .Param.name }}"}]`,
		`probes: [{"script": "echo $PARAM_name"}]`,
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
