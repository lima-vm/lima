package yqutil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/google/yamlfmt/formatters/basic"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/sirupsen/logrus"
	logging "gopkg.in/op/go-logging.v1"
)

// EvaluateExpression evaluates the yq expression, and returns the modified yaml.
func EvaluateExpression(expression string, content []byte) ([]byte, error) {
	logrus.Debugf("Evaluating yq expression: %q", expression)
	contentModified, err := replaceLineBreaksWithMagicString(content)
	if err != nil {
		return nil, err
	}
	tmpYAMLFile, err := os.CreateTemp("", "lima-yq-*.yaml")
	if err != nil {
		return nil, err
	}
	tmpYAMLPath := tmpYAMLFile.Name()
	defer os.RemoveAll(tmpYAMLPath)
	_, err = tmpYAMLFile.Write(contentModified)
	if err != nil {
		tmpYAMLFile.Close()
		return nil, err
	}
	if err = tmpYAMLFile.Close(); err != nil {
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
	out := new(bytes.Buffer)
	printer := yqlib.NewPrinter(encoder, yqlib.NewSinglePrinterWriter(out))
	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)

	streamEvaluator := yqlib.NewStreamEvaluator()
	files := []string{tmpYAMLPath}
	err = streamEvaluator.EvaluateFiles(expression, files, printer, decoder)
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

	return yamlfmt(out.Bytes())
}

func Join(yqExprs []string) string {
	if len(yqExprs) == 0 {
		return ""
	}
	return strings.Join(yqExprs, " | ")
}

func yamlfmt(content []byte) ([]byte, error) {
	factory := basic.BasicFormatterFactory{}
	config := map[string]interface{}{
		"indentless_arrays":  true,
		"line_ending":        "lf", // prefer LF even on Windows
		"pad_line_comments":  2,
		"retain_line_breaks": true, // does not affect to the output because yq removes empty lines before formatting
	}
	formatter, err := factory.NewFormatter(config)
	if err != nil {
		return nil, err
	}
	return formatter.Format(content)
}

const yamlfmtLineBreakPlaceholder = "#magic___^_^___line"

type paddinger struct {
	strings.Builder
}

func (p *paddinger) adjust(txt string) {
	var indentSize int
	for i := 0; i < len(txt) && txt[i] == ' '; i++ { // yaml only allows space to indent.
		indentSize++
	}
	// Grows if the given size is larger than us and always return the max padding.
	for diff := indentSize - p.Len(); diff > 0; diff-- {
		p.WriteByte(' ')
	}
}

func replaceLineBreaksWithMagicString(content []byte) ([]byte, error) {
	// hotfix: yq does not support line breaks in the middle of a string.
	var buf bytes.Buffer
	reader := bytes.NewReader(content)
	scanner := bufio.NewScanner(reader)
	var padding paddinger
	for scanner.Scan() {
		txt := scanner.Text()
		padding.adjust(txt)
		if strings.TrimSpace(txt) == "" { // line break or empty space line.
			buf.WriteString(padding.String()) // prepend some padding incase literal multiline strings.
			buf.WriteString(yamlfmtLineBreakPlaceholder)
			buf.WriteString("\n")
		} else {
			buf.WriteString(txt)
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), scanner.Err()
}
