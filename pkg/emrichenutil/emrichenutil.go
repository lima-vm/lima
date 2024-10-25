package emrichenutil

import (
	"bytes"
	"errors"
	"io"

	"github.com/lima-vm/lima/pkg/yamlutil"

	"github.com/go-go-golems/go-emrichen/pkg/emrichen"
	"gopkg.in/yaml.v3"
)

// EvaluateTemplate evaluates the emrichen template, and returns the modified yaml.
func EvaluateTemplate(data []byte) ([]byte, error) {
	var vars map[string]interface{}
	interpreter, err := emrichen.NewInterpreter(emrichen.WithVars(vars))
	if err != nil {
		return nil, err
	}
	formatter, err := yamlutil.NewFormatter()
	if err != nil {
		return nil, err
	}
	data, err = formatter.Before(data)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var resultNode *yaml.Node
	for {
		inputNode := yaml.Node{}
		// Parse input YAML
		err = decoder.Decode(interpreter.CreateDecoder(&inputNode))
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}

		// empty document node after defaults
		if inputNode.Kind == 0 {
			continue
		}
		resultNode, err = interpreter.Process(&inputNode)
		if err != nil {
			break
		}
	}
	output, err := yaml.Marshal(resultNode)
	if err != nil {
		return nil, err
	}
	return formatter.Format(output)
}
