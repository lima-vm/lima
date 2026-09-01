// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hcs

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

const (
	testDiskPath   = `C:\Users\lima\.lima\hcs\disk.vhdx`
	testISOPath    = `C:\Users\lima\.lima\hcs\cidata.iso`
	testEndpointID = "11111111-2222-3333-4444-555555555555"
	testMACAddress = "52-55-55-AA-BB-CC"
	testSerialPipe = `\\.\pipe\lima-hcs-com1`
	testCPUs       = 2
	testMemoryMiB  = 2048
)

func TestNetworkAndEndpointName(t *testing.T) {
	assert.Equal(t, networkName("test"), "lima-test-net")
	assert.Equal(t, endpointName("test"), "lima-test-ep")
}

func TestHNSMACAddress(t *testing.T) {
	dir := t.TempDir()
	got := hnsMACAddress(dir)

	re := regexp.MustCompile(`^52-55-55(-[0-9A-F]{2}){3}$`)
	assert.Assert(t, re.MatchString(got), "unexpected MAC address %#q", got)

	assert.Equal(t, got, hnsMACAddress(dir), "MAC Address should be identical if input is the same.")

	assert.Assert(t, got != hnsMACAddress(t.TempDir()))
}

func TestBuildVMConfig(t *testing.T) {
	got, err := buildVMConfig(testDiskPath, testISOPath, testEndpointID, testMACAddress, testSerialPipe, testCPUs, testMemoryMiB)
	assert.NilError(t, err)

	const want = `{
  "SchemaVersion": {"Major": 2, "Minor": 1},
  "Owner": "Lima",
  "ShouldTerminateOnLastHandleClosed": true,
  "VirtualMachine": {
    "Chipset": {
      "Uefi": {
        "Console": "ComPort1",
        "BootThis": {"DevicePath": "Primary disk", "DiskNumber": 0, "DeviceType": "ScsiDrive"}
      }
    },
    "ComputeTopology": {
      "Memory": {"Backing": "Virtual", "SizeInMB": 2048},
      "Processor": {"Count": 2}
    },
    "Devices": {
      "Scsi": {
        "Primary disk": {
          "Attachments": {"0": {"Type": "VirtualDisk", "Path": "C:\\Users\\lima\\.lima\\hcs\\disk.vhdx"}}
        },
        "cidata": {
          "Attachments": {"0": {"Type": "Iso", "Path": "C:\\Users\\lima\\.lima\\hcs\\cidata.iso"}}
        }
      },
      "ComPorts": {"0": {"NamedPipe": "\\\\.\\pipe\\lima-hcs-com1"}},
      "NetworkAdapters": {
        "11111111-2222-3333-4444-555555555555": {
          "EndpointId": "11111111-2222-3333-4444-555555555555",
          "MacAddress": "52-55-55-AA-BB-CC"
        }
      }
    }
  }
}`

	var gotDoc, wantDoc any
	assert.NilError(t, json.Unmarshal([]byte(got), &gotDoc))
	assert.NilError(t, json.Unmarshal([]byte(want), &wantDoc))
	assert.DeepEqual(t, gotDoc, wantDoc)
}

func TestBuildVMConfigSerialPortNumbering(t *testing.T) {
	got, err := buildVMConfig(testDiskPath, testISOPath, testEndpointID, testMACAddress, testSerialPipe, testCPUs, testMemoryMiB)
	assert.NilError(t, err)

	var doc struct {
		VirtualMachine struct {
			Chipset struct {
				Uefi struct {
					Console string
				}
			}
			Devices struct {
				ComPorts map[string]struct {
					NamedPipe string
				}
			}
		}
	}
	assert.NilError(t, json.Unmarshal([]byte(got), &doc))

	assert.Equal(t, doc.VirtualMachine.Chipset.Uefi.Console, "ComPort1")
	assert.Equal(t, doc.VirtualMachine.Devices.ComPorts["0"].NamedPipe, testSerialPipe)
}

func TestServeSerial(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "serial.log")
	ln := newFakeListener()

	done := make(chan struct{})
	go func() {
		serveSerial(ln, logPath)
		close(done)
	}()

	lines := []string{"[    0.000000] Linux version 6.11\n", "lima login: \n"}
	for _, line := range lines {
		guest, host := net.Pipe()
		ln.conns <- host
		_, err := guest.Write([]byte(line))
		assert.NilError(t, err)
		assert.NilError(t, guest.Close())
	}

	assert.NilError(t, ln.Close())
	<-done

	b, err := os.ReadFile(logPath)
	assert.NilError(t, err)
	assert.Equal(t, string(b), lines[0]+lines[1])
}

type fakeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newFakeListener() *fakeListener {
	return &fakeListener{
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
}

func (ln *fakeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-ln.conns:
		return conn, nil
	case <-ln.done:
		return nil, net.ErrClosed
	}
}

func (ln *fakeListener) Close() error {
	ln.once.Do(func() { close(ln.done) })
	return nil
}

func (ln *fakeListener) Addr() net.Addr { return fakeAddr{} }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }
