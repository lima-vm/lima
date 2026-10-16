// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	hostagentevents "github.com/lima-vm/lima/v2/pkg/hostagent/events"
)

func TestRenderRequirementProgressTTY(t *testing.T) {
	newLine := func() (*requirementProgressLine, *logrus.Logger, *bytes.Buffer) {
		out := &bytes.Buffer{}
		line := &requirementProgressLine{w: out}
		logger := logrus.New()
		logger.SetOutput(out)
		logger.SetFormatter(&progressLogFormatter{
			Formatter: &logrus.TextFormatter{DisableTimestamp: true},
			line:      line,
		})
		return line, logger, out
	}
	pending := &hostagentevents.RequirementProgress{Step: 1, Total: 2, Description: "Foo", Suffix: " (essential)"}
	done := &hostagentevents.RequirementProgress{Step: 1, Total: 2, Description: "Foo", Done: true}

	t.Run("log entry while pending", func(t *testing.T) {
		line, logger, out := newLine()
		var lastKey string

		renderRequirementProgress(pending, line, &lastKey)
		logger.Info("[hostagent] bar")
		renderRequirementProgress(done, line, &lastKey)

		assert.Equal(t, "\r\033[K( 1/2) 🕐 Foo (essential)"+
			"\r\033[Klevel=info msg=\"[hostagent] bar\"\n( 1/2) 🕐 Foo (essential)"+
			"\r\033[K( 1/2) ✅ Foo\n", out.String())
	})
	t.Run("log entry while not pending", func(t *testing.T) {
		line, logger, out := newLine()
		var lastKey string

		renderRequirementProgress(pending, line, &lastKey)
		renderRequirementProgress(done, line, &lastKey)
		out.Reset()
		logger.Info("[hostagent] bar")

		assert.Equal(t, "level=info msg=\"[hostagent] bar\"\n", out.String())
	})
	t.Run("end terminates pending line", func(t *testing.T) {
		line, _, out := newLine()
		var lastKey string

		renderRequirementProgress(pending, line, &lastKey)
		line.end()
		line.end()

		assert.Equal(t, "\r\033[K( 1/2) 🕐 Foo (essential)\n", out.String())
	})
}

func TestRenderRequirementProgressNonTTY(t *testing.T) {
	out := &bytes.Buffer{}
	logger := logrus.StandardLogger()
	origOut, origFormatter := logger.Out, logger.Formatter
	t.Cleanup(func() {
		logger.SetOutput(origOut)
		logger.SetFormatter(origFormatter)
	})
	logger.SetOutput(out)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	var lastKey string

	renderRequirementProgress(&hostagentevents.RequirementProgress{Step: 1, Total: 2, Description: "Foo", Suffix: " (essential)"}, nil, &lastKey)
	renderRequirementProgress(&hostagentevents.RequirementProgress{Step: 1, Total: 2, Description: "Foo", Done: true}, nil, &lastKey)
	renderRequirementProgress(&hostagentevents.RequirementProgress{Step: 2, Total: 2, Description: "Bar", Failed: true}, nil, &lastKey)

	assert.Equal(t, "level=info msg=\"( 1/2) 🕐 Foo (essential)\"\n"+
		"level=info msg=\"( 1/2) ✅ Foo\"\n"+
		"level=info msg=\"( 2/2) ❌ Bar\"\n", out.String())
}
