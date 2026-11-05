// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"

	pb "github.com/lima-vm/lima/v2/pkg/driver/external"
)

// relayWarnings prints the warnings relayed from the external driver via the gRPC trailer.
// The standard logrus logger is used, so that the warnings are printed on limactl output
// in the same format as other limactl logs.
func relayWarnings(trailer metadata.MD) {
	for _, s := range trailer.Get(pb.WarningsTrailerKey) {
		var w pb.Warning
		if err := json.Unmarshal([]byte(s), &w); err != nil {
			logrus.WithError(err).Debugf("Failed to unmarshal a warning relayed from the external driver: %q", s)
			continue
		}
		fields := make(logrus.Fields, len(w.Fields))
		for k, v := range w.Fields {
			fields[k] = v
		}
		logrus.WithFields(fields).Warn(w.Message)
	}
}
