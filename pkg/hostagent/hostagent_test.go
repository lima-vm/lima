// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestPickTCPDNSLocalPortSkipsSSHPort(t *testing.T) {
	const sshLocalPort = 60022
	ports := []int{sshLocalPort, sshLocalPort, 5353}
	freeTCP := func() (int, error) {
		port := ports[0]
		ports = ports[1:]
		return port, nil
	}
	port, err := pickTCPDNSLocalPort(sshLocalPort, freeTCP)
	assert.NilError(t, err)
	assert.Equal(t, port, 5353)
}

func TestPickTCPDNSLocalPortGivesUp(t *testing.T) {
	const sshLocalPort = 60022
	freeTCP := func() (int, error) { return sshLocalPort, nil }
	_, err := pickTCPDNSLocalPort(sshLocalPort, freeTCP)
	assert.ErrorContains(t, err, "other than the SSH port 60022")
}
