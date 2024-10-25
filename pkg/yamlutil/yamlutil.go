package yamlutil

import (
	"fmt"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/formatters/basic"
)

type Formatter struct {
	formatter *basic.BasicFormatter
}

// NewFormatter returns a basic formatter, to preserve indentation and empty lines.
func NewFormatter() (*Formatter, error) {
	formatter, err := yamlfmtBasicFormatter()
	if err != nil {
		return nil, err
	}
	return &Formatter{formatter: formatter}, nil
}

func (f *Formatter) Before(content []byte) ([]byte, error) {
	// `ApplyFeatures()` is being called directly before passing content to return.
	// This results in `ApplyFeatures()` being called twice with `FeatureApplyBefore`:
	// once here and once inside `formatter.Format`.
	// Currently, calling `ApplyFeatures()` with `FeatureApplyBefore` twice is not an issue,
	// but future changes to `yamlfmt` might cause problems if it is called twice.
	return f.formatter.Features.ApplyFeatures(content, yamlfmt.FeatureApplyBefore)
}

func (f *Formatter) Format(content []byte) ([]byte, error) {
	return f.formatter.Format(content)
}

func yamlfmtBasicFormatter() (*basic.BasicFormatter, error) {
	factory := basic.BasicFormatterFactory{}
	config := map[string]interface{}{
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
