package logrusutil

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const epsilon = 1 * time.Second

// PropagateJSON propagates JSONFormatter lines.
//
// PanicLevel and FatalLevel are converted to ErrorLevel.
func PropagateJSON(logger *logrus.Logger, jsonLine []byte, header string, begin time.Time) {
	if strings.TrimSpace(string(jsonLine)) == "" {
		return
	}

	var (
		lv  logrus.Level
		j   JSON
		err error
	)
	if err := json.Unmarshal(jsonLine, &j); err != nil {
		goto fallback
	}
	if !j.Time.IsZero() && !begin.IsZero() && begin.After(j.Time.Add(epsilon)) {
		return
	}
	lv, err = logrus.ParseLevel(j.Level)
	if err != nil {
		goto fallback
	}
	switch lv {
	case logrus.PanicLevel, logrus.FatalLevel:
		logger.WithField("level", lv).Error(header + j.Msg)
	case logrus.ErrorLevel:
		logger.Error(header + j.Msg)
	case logrus.WarnLevel:
		logger.Warn(header + j.Msg)
	case logrus.InfoLevel:
		logger.Info(header + j.Msg)
	case logrus.DebugLevel:
		logger.Debug(header + j.Msg)
	case logrus.TraceLevel:
		logger.Trace(header + j.Msg)
	}
	return

fallback:
	logrus.Info(header + string(jsonLine))
}

// JSON is the type used in logrus.JSONFormatter
type JSON struct {
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
	Time  time.Time `json:"time"`
}
