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
		entry  *logrus.Entry
		fields logrus.Fields
		lv     logrus.Level
		j      JSON
		err    error
	)
	entry = logrus.NewEntry(logger)

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
	entry = entry.WithTime(j.Time)
	// Unmarshal jsonLine once more to capture all the "extra" fields that have been added by
	// WithError() and WithField(). The regular fields "level", "msg", and "time" are already
	// unmarshalled into j and are handled specially. They must not be added again.
	if err := json.Unmarshal(jsonLine, &fields); err == nil {
		delete(fields, "level")
		delete(fields, "msg")
		delete(fields, "time")
		entry = entry.WithFields(fields)
	}
	// Don't exit on Fatal or Panic entries
	if lv <= logrus.FatalLevel {
		entry = entry.WithField("level", lv)
		lv = logrus.ErrorLevel
	}
	entry.Log(lv, header+j.Msg)
	return

fallback:
	entry.Info(header + string(jsonLine))
}

// JSON is the type used in logrus.JSONFormatter.
type JSON struct {
	Level string    `json:"level"`
	Msg   string    `json:"msg"`
	Time  time.Time `json:"time"`
}
