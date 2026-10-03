// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
)

func TestEmitCloudInitProgressEventStripsControlChars(t *testing.T) {
	var buf bytes.Buffer
	a := &HostAgent{eventEnc: json.NewEncoder(&buf)}

	// A compromised guest can put arbitrary bytes in its cloud-init log, including
	// an ANSI escape sequence that would rewrite the operator's terminal.
	const payload = "starting\x1b[2Kspoofed\x07 line"
	a.emitCloudInitProgressEvent(t.Context(), &events.CloudInitProgress{LogLine: payload})

	var ev events.Event
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &ev))
	assert.Assert(t, ev.Status.CloudInitProgress != nil)
	got := ev.Status.CloudInitProgress.LogLine
	for _, r := range got {
		assert.Assert(t, unicode.IsPrint(r), "control char %#q survived in %#q", r, got)
	}
	assert.Equal(t, got, "starting[2Kspoofed line")
	assert.Assert(t, !strings.ContainsRune(got, '\x1b'))
}
