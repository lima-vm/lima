// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnetinterface

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework vmnet
#include "vmnetinterface_darwin.h"
*/
import (
	"C" //nolint: gocritic // dupImport: package is imported 2 times under different aliases on lines.. (gocritic)
)

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"runtime/cgo"
	"strings"
	"syscall"
	"unsafe" //nolint: gocritic // dupImport: package is imported 2 times under different aliases on lines.. (gocritic)

	"github.com/Code-Hex/vz/v3/pkg/vmnet"
	"github.com/Code-Hex/vz/v3/pkg/xpc"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/vzvmnet"
)

const headerSize = unsafe.Sizeof(C.uint32_t(0))

// FileDescriptorForNetwork returns a file for the given vz network.
func FileDescriptorForNetwork(ctx context.Context, vzNetwork string) (*os.File, error) {
	// Load network configuration
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return nil, err
	}
	vmnetConfig, ok := nwCfg.Vz[vzNetwork]
	if !ok {
		return nil, fmt.Errorf("networks.yaml: 'vz: %s' is not defined", vzNetwork)
	}
	// Request VmnetNetwork
	network, err := vzvmnet.RequestVmnetNetwork(ctx, vzNetwork, vmnetConfig)
	if err != nil {
		return nil, fmt.Errorf("RequestVmnetNetwork failed: %w", err)
	}

	// Create socketpair connection as conn and file
	conn, file, err := connAndFile()
	if err != nil {
		return nil, fmt.Errorf("connAndFile failed: %w", err)
	}

	iface, err := startWithNetwork(network.Raw(), xpc.NewDictionary())
	if err != nil {
		return nil, fmt.Errorf("startWithNetwork failed: %w", err)
	}
	logrus.Debugf("[vmnet] started VmnetInterface with maxPacketSize=%d, maxReadPacketCount=%d, maxWritePacketCount=%d, param=%+v",
		iface.maxPacketSize, iface.maxReadPacketCount, iface.maxWritePacketCount, iface.param)

	go func() {
		logrus.Debug("[vmnet] vmnet interface goroutine started")
		defer C.VmnetStopInterface(iface.ptr)

		defer conn.Close()

		// Prepare vmPktDescs for reading packets from vmnet interface
		readDescs := newVMPktDescs(iface.maxReadPacketCount, iface.maxPacketSize)

		// Set packets available event callback to read packets from vmnet interface
		cgoHandle := cgo.NewHandle(packetsAvailableEventCallback(func(estimatedCount C.int) {
			for estimatedCount > 0 {
				var packetCount C.int
				// Read packets from vmnet interface
				if packetCount, err = iface.ReadPackets(readDescs, estimatedCount); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
					logrus.WithError(err).Error("iface.ReadPackets failed")
					return
				}
				// To use built-in Writev implementation in net package (internal/poll.FD.Writev),
				// we use net.Buffers and its WriteTo method.
				if writeBuffers, err := readDescs.buffersForWritingToConn(packetCount); err != nil {
					logrus.WithError(err).Error("buffersForWritingToConn failed")
					return
				} else if _, err := writeBuffers.WriteTo(conn); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
					logrus.WithError(err).Error("writeBuffers.WriteTo failed")
					return
				}
				estimatedCount -= packetCount
			}
		}))
		defer cgoHandle.Delete()
		if err := iface.SetPacketsAvailableEventCallback(cgoHandle); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
			logrus.WithError(err).Error("SetPacketsAvailableEventCallback failed")
			return
		}
		// Start reading packet from the connection (VM) and writing to vmnet interface.
		// Packets comes one by one with 4-byte big-endian header indicating the packet size.
		// Read all available packets in a loop.
		writeDescs := newVMPktDescs(iface.maxWritePacketCount, iface.maxPacketSize)
		for {
			// Read packets from the connection to writeDescs
			packetCount, err := writeDescs.readPacketsFromConn(conn)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					// Normal closure
					break
				}
				logrus.WithError(err).Error("writeDescs.readPacketsFromConn failed")
				break
			}
			// Write packets to vmnet interface
			if writtenCount, err := iface.WritePackets(writeDescs, packetCount); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
				logrus.WithError(err).Errorf("iface.WritePackets failed with packetCount=%d, writtenCount=%d", packetCount, writtenCount)
				break
			}
		}
		// Keep readBuffers and writeBuffers alive until the goroutine ends
		runtime.KeepAlive(readDescs)
		runtime.KeepAlive(writeDescs)
	}()

	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
			if !strings.Contains(err.Error(), "use of closed network connection") {
				logrus.WithError(err).Error("failed to close conn on context done")
			}
		}
	}()

	return file, nil
}

// connAndFile creates a socketpair and returns one end as net.Conn and the other as *os.File.
func connAndFile() (net.Conn, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create socketpair: %w", err)
	}
	if err := syscall.SetNonblock(fds[0], true); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
		syscall.Close(fds[0])
		syscall.Close(fds[1])
		return nil, nil, fmt.Errorf("failed to set nonblock on socketpair[0]: %w", err)
	}
	if err := syscall.SetNonblock(fds[1], true); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
		syscall.Close(fds[0])
		syscall.Close(fds[1])
		return nil, nil, fmt.Errorf("failed to set nonblock on socketpair[1]: %w", err)
	}

	connFile := os.NewFile(uintptr(fds[0]), "vmnet-conn")
	conn, err := net.FileConn(connFile)
	if err != nil {
		connFile.Close()
		syscall.Close(fds[1])
		return nil, nil, fmt.Errorf("failed to create FileConn: %w", err)
	}
	if err = connFile.Close(); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
		conn.Close()
		syscall.Close(fds[1])
		return nil, nil, fmt.Errorf("failed to close connFile: %w", err)
	}
	file := os.NewFile(uintptr(fds[1]), "vmnet-file")
	return conn, file, nil
}

// iface represents a VmnetInterface instance.
type iface struct {
	ptr                 unsafe.Pointer
	param               *xpc.Dictionary
	maxPacketSize       C.uint64_t
	maxReadPacketCount  C.int
	maxWritePacketCount C.int
}

// SetPacketsAvailableEventCallback sets the packets available event callback.
func (i *iface) SetPacketsAvailableEventCallback(callback cgo.Handle) error {
	if result := vmnet.Return(
		C.VmnetInterfaceSetPacketsAvailableEventCallback(i.ptr, C.uintptr_t(callback)),
	); result != vmnet.ErrSuccess {
		return fmt.Errorf("VmnetInterfaceSetPacketsAvailableEventCallback failed: %w", result)
	}
	return nil
}

// ReadPackets reads packets from the VmnetInterface into vmPktDescs.
// It returns the number of packets read.
func (i *iface) ReadPackets(v *vmPktDescs, packetCount C.int) (C.int, error) {
	// Reset vmPktDescs before reading
	v.reset()
	// Limit packetCount to maxReadPacketCount
	count := min(packetCount, i.maxReadPacketCount)
	if result := vmnet.Return(C.VmnetRead(i.ptr, v.self, &count)); result != vmnet.ErrSuccess {
		return 0, fmt.Errorf("VmnetRead failed: %w", result)
	}
	return count, nil
}

// WritePackets writes packets to the VmnetInterface from vmPktDescs.
// It returns the number of packets written.
func (i *iface) WritePackets(v *vmPktDescs, packetCount C.int) (C.int, error) {
	if result := vmnet.Return(C.VmnetWrite(i.ptr, v.self, &packetCount)); result != vmnet.ErrSuccess {
		// Will partial write happen here?
		return 0, fmt.Errorf("VmnetWrite failed: %w", result)
	}
	return packetCount, nil
}

// startWithNetwork starts a VmnetInterface with the given VmnetNetwork and interface description.
func startWithNetwork(network unsafe.Pointer, interfaceDesc *xpc.Dictionary) (*iface, error) {
	result := C.VmnetInterfaceStartWithNetwork(network, interfaceDesc.Raw())
	if vmnetResult := vmnet.Return(result.vmnetReturn); vmnetResult != vmnet.ErrSuccess {
		return nil, fmt.Errorf("VmnetInterfaceStartWithNetwork failed: %w", vmnetResult)
	}
	i := &iface{
		ptr:                 result.iface,
		param:               xpc.ReleaseOnCleanup(xpc.NewObject(result.ifaceParam).(*xpc.Dictionary)),
		maxPacketSize:       result.maxPacketSize,
		maxReadPacketCount:  result.maxReadPacketCount,
		maxWritePacketCount: result.maxWritePacketCount,
	}
	runtime.AddCleanup(i, func(ptr unsafe.Pointer) { C.VmnetReleaseInterface(ptr) }, i.ptr)
	return i, nil
}

type packetsAvailableEventCallback func(estimatedCount C.int)

//export callPacketsAvailableEventCallback
func callPacketsAvailableEventCallback(cgoHandle uintptr, estimatedCount C.int) {
	if cgoHandle != 0 {
		callback := cgo.Handle(cgoHandle).Value().(packetsAvailableEventCallback)
		callback(estimatedCount)
	}
}

type vmPktDescs struct {
	self           *C.struct_vmpktdesc
	buffers        net.Buffers
	maxPacketCount C.int
	maxPacketSize  C.uint64_t
}

// newVMPktDescs allocates VMPktDesc array and backing buffers.
// VMPktDesc's iov_base points to the buffer after 4-byte header.
// The 4-byte header is reserved for packet size to the connection.
func newVMPktDescs(count C.int, maxPacketSize C.uint64_t) *vmPktDescs {
	v := &vmPktDescs{
		self:           C.allocateVMPktDescArray(count, maxPacketSize),
		buffers:        make(net.Buffers, 0, int(count)),
		maxPacketCount: count,
		maxPacketSize:  maxPacketSize,
	}
	runtime.AddCleanup(v, func(self *C.struct_vmpktdesc) { C.deallocateVMPktDescArray(self) }, v.self)
	for i := range int(count) {
		// Allocate buffer with extra 4 bytes for header
		buf := make([]byte, 0, maxPacketSize+C.uint64_t(headerSize))
		vmPktDesc := v.at(i)
		// point after the 4-byte header
		vmPktDesc.vm_pkt_iov.iov_base = unsafe.Add(unsafe.Pointer(unsafe.SliceData(buf)), headerSize)
		vmPktDesc.vm_pkt_iov.iov_len = C.size_t(maxPacketSize)
		v.buffers = append(v.buffers, buf)
	}
	return v
}

// at returns the pointer to the vmPktDesc at the given index.
func (v *vmPktDescs) at(index int) *C.struct_vmpktdesc {
	return (*C.struct_vmpktdesc)(unsafe.Add(unsafe.Pointer(v.self), index*int(unsafe.Sizeof(C.struct_vmpktdesc{}))))
}

// reset resets vmPktDescs to initial state.
func (v *vmPktDescs) reset() {
	C.resetVMPktDescArray(v.self, v.maxPacketCount, v.maxPacketSize)
}

// buffersForWritingToConn returns net.Buffers to write to the connection
// adjusted their buffer sizes based vm_pkt_size in vmPktDescs read from vmnet interface.
func (v *vmPktDescs) buffersForWritingToConn(packetCount C.int) (net.Buffers, error) {
	bufs := make(net.Buffers, 0, int(packetCount))
	for i := range int(packetCount) {
		vmPktDesc := v.at(i)
		if C.uint64_t(vmPktDesc.vm_pkt_size) > v.maxPacketSize {
			return nil, fmt.Errorf("vm_pkt_size %d exceeds maxPacketSize %d", vmPktDesc.vm_pkt_size, v.maxPacketSize)
		}
		// Write packet size to the 4-byte header
		binary.BigEndian.PutUint32(v.buffers[i][:headerSize], uint32(vmPktDesc.vm_pkt_size))
		// Resize buffer to include header and packet size
		bufs = append(bufs, v.buffers[i][:headerSize+uintptr(vmPktDesc.vm_pkt_size)])
	}
	return bufs, nil
}

// readPacketsFromConn reads packets from the connection into vmPktDescs.
//   - It returns the number of packets read.
//   - The packets are expected to come one by one with 4-byte big-endian header indicating the packet size.
//   - It reads all available packets until no more packets are available, packetCount reaches maxPacketCount, or an error occurs.
//   - It waits for the connection to be ready for initial read of 4-byte header.
func (v *vmPktDescs) readPacketsFromConn(conn net.Conn) (C.int, error) {
	var packetCount int
	// Wait until 4-byte header is read
	if _, err := conn.Read(v.buffers[packetCount][:headerSize]); err != nil { //nolint: gocritic // https://github.com/golangci/golangci-lint/issues/3628
		return 0, fmt.Errorf("conn.Read failed: %w", err)
	}
	// Get rawConn for Readv
	rawConn, _ := conn.(syscall.Conn).SyscallConn()
	// Read available packets
	var packetLen uint32
	var bufs net.Buffers
	for {
		packetLen = binary.BigEndian.Uint32(v.buffers[packetCount][:headerSize])
		if packetLen == 0 || C.uint64_t(packetLen) > v.maxPacketSize {
			return 0, fmt.Errorf("invalid packetLen: %d (max %d)", packetLen, v.maxPacketSize)
		}

		// prepare buffers for reading packet and next header if any
		if packetCount+1 < int(v.maxPacketCount) {
			// prepare next header read as well
			bufs = net.Buffers{
				v.buffers[packetCount][headerSize : headerSize+uintptr(packetLen)],
				v.buffers[packetCount+1][:headerSize],
			}
		} else {
			// prepare only packet read to avoid exceeding maxPacketCount
			bufs = net.Buffers{
				v.buffers[packetCount][headerSize : headerSize+uintptr(packetLen)],
			}
		}

		// Read packet from the connection
		var bytesHasBeenRead int
		var err error
		rawConnReadErr := rawConn.Read(func(fd uintptr) (done bool) {
			// read packet into buffers
			bytesHasBeenRead, err = unix.Readv(int(fd), bufs)
			if bytesHasBeenRead <= 0 {
				if errors.Is(err, syscall.EAGAIN) {
					return false // try again later
				}
				err = fmt.Errorf("unix.Readv failed: %w", err)
				return true
			}
			// assumes partial read of a packet does not happen since packet len is already known
			return true
		})
		if rawConnReadErr != nil {
			return 0, fmt.Errorf("rawConn.Read failed: %w", rawConnReadErr)
		}
		if err != nil {
			return 0, fmt.Errorf("closure in rawConn.Read failed: %w", err)
		}
		packetCount++
		if bytesHasBeenRead == int(packetLen) {
			// next packet seems not available now, or reached maxPacketCount
			break
		} else if bytesHasBeenRead != int(packetLen)+int(headerSize) {
			return 0, fmt.Errorf("unexpected bytesHasBeenRead: %d (expected %d or %d)", bytesHasBeenRead, packetLen, packetLen+uint32(headerSize))
		}
	}
	// Update writeDescs based on writeDescs.buffers
	if err := v.updateForWritingToVmnet(packetCount); err != nil {
		return 0, fmt.Errorf("updateForWritingToVmnet failed: %w", err)
	}
	return C.int(packetCount), nil
}

// updateForWritingToVmnet updates vmPktDescs for writing based on buffers before writing to vmnet interface.
func (v *vmPktDescs) updateForWritingToVmnet(packetCount int) error {
	v.reset()
	for i := range packetCount {
		vmPktDesc := v.at(i)
		packetLen := binary.BigEndian.Uint32(v.buffers[i][:headerSize])
		if packetLen == 0 {
			return fmt.Errorf("invalid packetLen: %d", packetLen)
		}
		if C.uint64_t(packetLen) > v.maxPacketSize {
			return fmt.Errorf("packetLen %d exceeds maxPacketSize %d", packetLen, v.maxPacketSize)
		}
		// Update vmPktDesc's size
		vmPktDesc.vm_pkt_size = C.size_t(packetLen)
		// vmnet requires to sum of iov_len in vm_pkt_iov to be equal to vm_pkt_size
		vmPktDesc.vm_pkt_iov.iov_len = C.size_t(packetLen)
	}
	return nil
}
