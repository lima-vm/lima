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

package yqutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/formatters/basic"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/sirupsen/logrus"
	logging "gopkg.in/op/go-logging.v1"
)

// ValidateContent decodes the content yaml, to check it for syntax errors.
func ValidateContent(content []byte) error {
	memory := logging.NewMemoryBackend(0)
	backend := logging.AddModuleLevel(memory)
	logging.SetBackend(backend)
	yqlib.InitExpressionParser()

	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)

	reader := bytes.NewReader(content)
	err := decoder.Init(reader)
	if err != nil {
		return err
	}
	_, err = decoder.Decode()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

// EvaluateExpression evaluates the yq expression and returns the modified yaml.
func EvaluateExpression(expression string, content []byte) ([]byte, error) {
	if expression == "" {
		return content, nil
	}
	logrus.Debugf("Evaluating yq expression: %q", expression)
	formatter, err := yamlfmtBasicFormatter()
	if err != nil {
		return nil, err
	}
	// `ApplyFeatures()` is being called directly before passing content to `yqlib`.
	// This results in `ApplyFeatures()` being called twice with `FeatureApplyBefore`:
	// once here and once inside `formatter.Format`.
	// Currently, calling `ApplyFeatures()` with `FeatureApplyBefore` twice is not an issue,
	// but future changes to `yamlfmt` might cause problems if it is called twice.
	_, contentModified, err := formatter.Features.ApplyFeatures(context.Background(), content, yamlfmt.FeatureApplyBefore)
	if err != nil {
		return nil, err
	}
	memory := logging.NewMemoryBackend(0)
	backend := logging.AddModuleLevel(memory)
	logging.SetBackend(backend)
	yqlib.InitExpressionParser()

	encoderPrefs := yqlib.ConfiguredYamlPreferences.Copy()
	encoderPrefs.Indent = 2
	encoderPrefs.ColorsEnabled = false
	encoder := yqlib.NewYamlEncoder(encoderPrefs)
	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)
	out, err := yqlib.NewStringEvaluator().EvaluateAll(expression, string(contentModified), encoder, decoder)
	if err != nil {
		logger := logrus.StandardLogger()
		for node := memory.Head(); node != nil; node = node.Next() {
			entry := logrus.NewEntry(logger).WithTime(node.Record.Time)
			prefix := fmt.Sprintf("[%s] ", node.Record.Module)
			message := prefix + node.Record.Message()
			switch node.Record.Level {
			case logging.CRITICAL:
				entry.Fatal(message)
			case logging.ERROR:
				entry.Error(message)
			case logging.WARNING:
				entry.Warn(message)
			case logging.NOTICE:
				entry.Info(message)
			case logging.INFO:
				entry.Info(message)
			case logging.DEBUG:
				entry.Debug(message)
			}
		}
		return nil, err
	}
	return formatter.Format([]byte(out))
}

func Join(yqExprs []string) string {
	return strings.Join(yqExprs, " | ")
}

func yamlfmtBasicFormatter() (*basic.BasicFormatter, error) {
	factory := basic.BasicFormatterFactory{}
	config := map[string]any{
		"indentless_arrays":         true,
		"line_ending":               "lf", // prefer LF even on Windows
		"pad_line_comments":         2,
		"retain_line_breaks":        true,
		"retain_line_breaks_single": false,
	}

	formatter, err := factory.NewFormatter(config)
	if err != nil {
		return nil, err
	}
	basicFormatter, ok := formatter.(*basic.BasicFormatter)
	if !ok {
		return nil, fmt.Errorf("unexpected formatter type: %T", formatter)
	}
	return basicFormatter, nil
}
