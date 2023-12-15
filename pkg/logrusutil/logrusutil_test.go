package logrusutil

import (
	"bytes"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestPropagateJSON(t *testing.T) {
	loggerWithoutTs := func(output *bytes.Buffer) *logrus.Logger {
		logger := logrus.New()
		logger.SetOutput(output)
		logger.SetLevel(logrus.TraceLevel)
		logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
		return logger
	}

	t.Run("trace level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "trace"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=trace msg=header\n", actual.String())
	})
	t.Run("debug level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "debug"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=debug msg=header\n", actual.String())
	})
	t.Run("info level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "info"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=info msg=header\n", actual.String())
	})
	t.Run("error level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "error"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=error msg=header\n", actual.String())
	})
	t.Run("warning level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "warning"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=warning msg=header\n", actual.String())
	})
	t.Run("panic level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "panic"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=error msg=header fields.level=panic\n", actual.String())
	})
	t.Run("fatal level", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "fatal"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "level=error msg=header fields.level=fatal\n", actual.String())
	})
	t.Run("empty json line", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte{}

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, "", actual.String())
	})
	t.Run("unmarshal failed", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`"`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, `level=info msg="header\""
`, actual.String())
	})
	t.Run("begin time after time in jsonLine", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "info", "time": "2023-12-01T00:00:00.0000+00:00"}`)
		begin := time.Date(2023, time.December, 15, 0, 0, 0, 0, time.UTC)

		PropagateJSON(logger, jsonLine, "header", begin)

		assert.Equal(t, "", actual.String())
	})
	t.Run("parse level failed", func(t *testing.T) {
		actual := &bytes.Buffer{}
		logger := loggerWithoutTs(actual)
		jsonLine := []byte(`{"level": "info", "level": "unknown level"}`)

		PropagateJSON(logger, jsonLine, "header", time.Time{})

		assert.Equal(t, `level=info msg="header{\"level\": \"info\", \"level\": \"unknown level\"}"
`, actual.String())
	})
}
