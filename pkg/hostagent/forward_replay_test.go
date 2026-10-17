// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"net"
	"slices"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func newReplayTestPortForwarder(master *sshMaster) *portForwarder {
	rules := []limatype.PortForward{{
		GuestIP:           net.IPv4(127, 0, 0, 1),
		GuestIPMustBeZero: new(false),
		GuestPortRange:    [2]int{1024, 65535},
		HostIP:            net.IPv4(127, 0, 0, 1),
		HostPortRange:     [2]int{1024, 65535},
		Proto:             limatype.ProtoTCP,
	}}
	sshAddressPort := func() (string, int) { return "127.0.0.1", 60022 }
	return newPortForwarder(nil, sshAddressPort, rules, false, "", master, nil)
}

func tcpPort(port int32) *api.IPPort {
	return &api.IPPort{Ip: "127.0.0.1", Port: port, Protocol: "tcp"}
}

func TestPortForwarderReplay(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	master := &sshMaster{gen: 1}
	pf := newReplayTestPortForwarder(master)
	pf.OnEvent(t.Context(), &api.Event{AddedLocalPorts: []*api.IPPort{tcpPort(8080), tcpPort(8081)}})
	pf.OnEvent(t.Context(), &api.Event{RemovedLocalPorts: []*api.IPPort{tcpPort(8081)}})
	pf.forwardStatic(t.Context(), "127.0.0.1:9090", "127.0.0.1:9090")
	before := len(calls())

	// Every intent is on the current generation, so nothing is replayed.
	pf.replay(t.Context(), 1)
	assert.Equal(t, len(calls()), before)

	master.gen = 2
	pf.replay(t.Context(), 2)
	got := calls()[before:]
	assert.Equal(t, len(got), 4, "a cancel and a forward each for 8080 and the static 9090; none for the removed 8081")
	byLocal := map[string][]string{}
	for _, c := range got {
		byLocal[c.local] = append(byLocal[c.local], c.verb)
	}
	assert.DeepEqual(t, byLocal, map[string][]string{
		"127.0.0.1:8080": {verbCancel, verbForward},
		"127.0.0.1:9090": {verbCancel, verbForward},
	})

	// A second replay of the same generation does nothing.
	pf.replay(t.Context(), 2)
	assert.Equal(t, len(calls()), before+4)
}

func TestPortForwarderReplay_IntentWithoutMaster(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	// A port added before the first master is ready is forwarded by the
	// replay of generation 1.
	master := &sshMaster{}
	pf := newReplayTestPortForwarder(master)
	pf.OnEvent(t.Context(), &api.Event{AddedLocalPorts: []*api.IPPort{tcpPort(8080)}})
	before := len(calls())
	master.gen = 1
	pf.replay(t.Context(), 1)
	assert.Equal(t, len(calls()), before+2)
}

func TestPortForwarderReplay_AfterStopRestarting(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	master := newSSHMaster(nil, "", "127.0.0.1", 60022)
	master.gen = 1
	pf := newReplayTestPortForwarder(master)
	pf.OnEvent(t.Context(), &api.Event{AddedLocalPorts: []*api.IPPort{tcpPort(8080)}})
	before := len(calls())
	master.stopRestarting()
	pf.replay(t.Context(), 2)
	assert.Equal(t, len(calls()), before)
}

func newReplayTestHostAgent(master *sshMaster) *HostAgent {
	return &HostAgent{
		master:        master,
		portForwarder: newReplayTestPortForwarder(master),
		instConfig: &limatype.LimaYAML{PortForwards: []limatype.PortForward{
			{GuestSocket: "/run/a.sock", HostSocket: "/tmp/a.sock"},
			{GuestPort: 8080},
			{GuestSocket: "/run/b.sock", HostSocket: "/tmp/b.sock", Reverse: true},
		}},
	}
}

func TestGuestSocketReplay(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	master := &sshMaster{gen: 1}
	a := newReplayTestHostAgent(master)
	a.forwardGuestSockets(t.Context())
	got := calls()
	assert.Assert(t, slices.Equal(got, []forwardCall{{verbForward, "/tmp/a.sock"}, {verbForward, "/tmp/b.sock"}}), "got %v", got)

	a.replayForwards(t.Context(), 1)
	assert.Equal(t, len(calls()), 2, "forwards made on the current generation are not replayed")

	master.gen = 2
	a.replayForwards(t.Context(), 2)
	byLocal := map[string][]string{}
	for _, c := range calls()[2:] {
		byLocal[c.local] = append(byLocal[c.local], c.verb)
	}
	assert.DeepEqual(t, byLocal, map[string][]string{
		"/tmp/a.sock": {verbCancel, verbForward},
		"/tmp/b.sock": {verbCancel, verbForward},
	})
}

func TestGuestSocketReplay_AfterStopRestarting(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	master := newSSHMaster(nil, "", "127.0.0.1", 60022)
	master.gen = 1
	a := newReplayTestHostAgent(master)
	a.forwardGuestSockets(t.Context())
	assert.NilError(t, a.cancelGuestSockets(t.Context()))
	before := len(calls())
	master.stopRestarting()
	a.replayForwards(t.Context(), 2)
	assert.Equal(t, len(calls()), before)
}

func TestForwardsWithoutSSHMaster(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	// On Windows hosts the host agent does not supervise a master, so master is nil.
	a := newReplayTestHostAgent(nil)
	a.forwardGuestSockets(t.Context())
	a.portForwarder.forwardStatic(t.Context(), "127.0.0.1:9090", "127.0.0.1:9090")
	a.portForwarder.OnEvent(t.Context(), &api.Event{AddedLocalPorts: []*api.IPPort{tcpPort(8080)}})
	before := len(calls())
	assert.Equal(t, before, 4, "two guestSocket forwards, the static 9090 and 8080")

	a.replayForwards(t.Context(), 1)
	assert.Equal(t, len(calls()), before)
}
