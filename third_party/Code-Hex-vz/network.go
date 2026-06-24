package vz

/*
#cgo darwin CFLAGS: -mmacosx-version-min=11 -x objective-c -fno-objc-arc
#cgo darwin LDFLAGS: -lobjc -framework Foundation -framework Virtualization
# include "virtualization_11.h"
# include "virtualization_13.h"
*/
import "C"
import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/Code-Hex/vz/v3/internal/objc"
)

// BridgedNetwork defines a network interface that bridges a physical interface with a virtual machine.
//
// A bridged interface is shared between the virtual machine and the host system. Both host and
// virtual machine send and receive packets on the same physical interface but have distinct network layers.
//
// The BridgedNetwork can be used with a BridgedNetworkDeviceAttachment to set up a network device NetworkDeviceConfiguration.
// see: https://developer.apple.com/documentation/virtualization/vzbridgednetworkinterface?language=objc
type BridgedNetwork interface {
	objc.NSObject

	// NetworkInterfaces returns the list of network interfaces available for bridging.
	NetworkInterfaces() []BridgedNetwork

	// Identifier returns the unique identifier for this interface.
	// The identifier is the BSD name associated with the interface (e.g. "en0").
	Identifier() string

	// LocalizedDisplayName returns a display name if available (e.g. "Ethernet").
	LocalizedDisplayName() string
}

// NewBridgedNetwork creates a new BridgedNetwork with identifier.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NetworkInterfaces() []BridgedNetwork {
	nsArray := objc.NewNSArray(
		C.VZBridgedNetworkInterface_networkInterfaces(),
	)
	ptrs := nsArray.ToPointerSlice()
	networkInterfaces := make([]BridgedNetwork, len(ptrs))
	for i, ptr := range ptrs {
		networkInterfaces[i] = &baseBridgedNetwork{
			pointer: objc.NewPointer(ptr),
		}
	}
	return networkInterfaces
}

type baseBridgedNetwork struct {
	*pointer
}

func (*baseBridgedNetwork) NetworkInterfaces() []BridgedNetwork {
	return NetworkInterfaces()
}

// Identifier returns the unique identifier for this interface.
//
// The identifier is the BSD name associated with the interface (e.g. "en0").
func (b *baseBridgedNetwork) Identifier() string {
	cstring := (*char)(C.VZBridgedNetworkInterface_identifier(objc.Ptr(b)))
	return cstring.String()
}

// LocalizedDisplayName returns a display name if available (e.g. "Ethernet").
//
// If no display name is available, the identifier is returned.
func (b *baseBridgedNetwork) LocalizedDisplayName() string {
	cstring := (*char)(C.VZBridgedNetworkInterface_localizedDisplayName(objc.Ptr(b)))
	return cstring.String()
}

// Network device attachment using network address translation (NAT) with outside networks.
//
// Using the NAT attachment type, the host serves as router and performs network address translation
// for accesses to outside networks.
// see: https://developer.apple.com/documentation/virtualization/vznatnetworkdeviceattachment?language=objc
type NATNetworkDeviceAttachment struct {
	*pointer

	*baseNetworkDeviceAttachment
}

func (*NATNetworkDeviceAttachment) String() string {
	return "NATNetworkDeviceAttachment"
}

var _ NetworkDeviceAttachment = (*NATNetworkDeviceAttachment)(nil)

// NewNATNetworkDeviceAttachment creates a new NATNetworkDeviceAttachment.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewNATNetworkDeviceAttachment() (*NATNetworkDeviceAttachment, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	attachment := &NATNetworkDeviceAttachment{
		pointer: objc.NewPointer(C.newVZNATNetworkDeviceAttachment()),
	}
	objc.SetFinalizer(attachment, func(self *NATNetworkDeviceAttachment) {
		objc.Release(self)
	})
	return attachment, nil
}

// BridgedNetworkDeviceAttachment represents a physical interface on the host computer.
//
// Use this struct when configuring a network interface for your virtual machine.
// A bridged network device sends and receives packets on the same physical interface
// as the host computer, but does so using a different network layer.
//
// To use this attachment, your app must have the com.apple.vm.networking entitlement.
// If it doesn’t, the use of this attachment point results in an invalid VZVirtualMachineConfiguration object in objective-c.
//
// see: https://developer.apple.com/documentation/virtualization/vzbridgednetworkdeviceattachment?language=objc
type BridgedNetworkDeviceAttachment struct {
	*pointer

	*baseNetworkDeviceAttachment
}

func (*BridgedNetworkDeviceAttachment) String() string {
	return "BridgedNetworkDeviceAttachment"
}

var _ NetworkDeviceAttachment = (*BridgedNetworkDeviceAttachment)(nil)

// NewBridgedNetworkDeviceAttachment creates a new BridgedNetworkDeviceAttachment with networkInterface.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewBridgedNetworkDeviceAttachment(networkInterface BridgedNetwork) (*BridgedNetworkDeviceAttachment, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	attachment := &BridgedNetworkDeviceAttachment{
		pointer: objc.NewPointer(
			C.newVZBridgedNetworkDeviceAttachment(
				objc.Ptr(networkInterface),
			),
		),
	}
	objc.SetFinalizer(attachment, func(self *BridgedNetworkDeviceAttachment) {
		objc.Release(self)
	})
	return attachment, nil
}

// FileHandleNetworkDeviceAttachment sending raw network packets over a file handle.
//
// The file handle attachment transmits the raw packets/frames between the virtual network interface and a file handle.
// The data transmitted through this attachment is at the level of the data link layer.
// see: https://developer.apple.com/documentation/virtualization/vzfilehandlenetworkdeviceattachment?language=objc
type FileHandleNetworkDeviceAttachment struct {
	*pointer

	*baseNetworkDeviceAttachment

	mtu int
}

func (*FileHandleNetworkDeviceAttachment) String() string {
	return "FileHandleNetworkDeviceAttachment"
}

var _ NetworkDeviceAttachment = (*FileHandleNetworkDeviceAttachment)(nil)

// NewFileHandleNetworkDeviceAttachment initialize the attachment with a file handle.
//
// file parameter is holding a connected datagram socket.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewFileHandleNetworkDeviceAttachment(file *os.File) (*FileHandleNetworkDeviceAttachment, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}
	err := validateDatagramSocket(int(file.Fd()))
	if err != nil {
		return nil, err
	}

	attachment := &FileHandleNetworkDeviceAttachment{
		pointer: objc.NewPointer(
			C.newVZFileHandleNetworkDeviceAttachment(
				C.int(file.Fd()),
			),
		),
		mtu: 1500, // The default MTU is 1500.
	}
	objc.SetFinalizer(attachment, func(self *FileHandleNetworkDeviceAttachment) {
		objc.Release(self)
	})
	return attachment, nil
}

func validateDatagramSocket(fd int) error {
	sotype, err := syscall.GetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_TYPE,
	)
	if err != nil {
		return os.NewSyscallError("getsockopt", err)
	}
	if sotype == syscall.SOCK_DGRAM && isAvailableDatagram(fd) {
		return nil
	}
	return fmt.Errorf("the fileHandle must be a datagram socket")
}

func isAvailableDatagram(fd int) bool {
	lsa, _ := syscall.Getsockname(fd)
	switch lsa.(type) {
	case *syscall.SockaddrInet4, *syscall.SockaddrInet6, *syscall.SockaddrUnix:
		return true
	}
	return false
}

// SetMaximumTransmissionUnit sets the maximum transmission unit (MTU) associated with this attachment.
//
// The maximum MTU allowed is 65535, and the minimum MTU allowed is 1500. An invalid MTU value will result in an invalid
// virtual machine configuration.
//
// The client side of the associated datagram socket must be properly configured with the appropriate values
// for SO_SNDBUF, and SO_RCVBUF. Set these using the setsockopt(_:_:_:_:_:) system call. The system expects
// the value of SO_RCVBUF to be at least double the value of SO_SNDBUF, and for optimal performance, the
// recommended value of SO_RCVBUF is four times the value of SO_SNDBUF.
//
// This is only supported on macOS 13 and newer, error will
// be returned on older versions.
func (f *FileHandleNetworkDeviceAttachment) SetMaximumTransmissionUnit(mtu int) error {
	if err := macOSAvailable(13); err != nil {
		return err
	}
	C.setMaximumTransmissionUnitVZFileHandleNetworkDeviceAttachment(
		objc.Ptr(f),
		C.NSInteger(mtu),
	)
	f.mtu = mtu
	return nil
}

// MaximumTransmissionUnit returns the maximum transmission unit (MTU) associated with this attachment.
// The default MTU is 1500.
func (f *FileHandleNetworkDeviceAttachment) MaximumTransmissionUnit() int {
	return f.mtu
}

// NetworkDeviceAttachment for a network device attachment.
// see: https://developer.apple.com/documentation/virtualization/vznetworkdeviceattachment?language=objc
type NetworkDeviceAttachment interface {
	objc.NSObject
	fmt.Stringer
	networkDeviceAttachment()
}

type baseNetworkDeviceAttachment struct{}

func (*baseNetworkDeviceAttachment) networkDeviceAttachment() {}

// VirtioNetworkDeviceConfiguration is configuration of a paravirtualized network device of type Virtio Network Device.
//
// The communication channel used on the host is defined through the attachment.
// It is set with the VZNetworkDeviceConfiguration.attachment property in objective-c.
//
// The configuration is only valid with valid MACAddress and attachment.
//
// see: https://developer.apple.com/documentation/virtualization/vzvirtionetworkdeviceconfiguration?language=objc
type VirtioNetworkDeviceConfiguration struct {
	*pointer

	attachment NetworkDeviceAttachment
}

// NewVirtioNetworkDeviceConfiguration creates a new VirtioNetworkDeviceConfiguration with NetworkDeviceAttachment.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewVirtioNetworkDeviceConfiguration(attachment NetworkDeviceAttachment) (*VirtioNetworkDeviceConfiguration, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	config := newVirtioNetworkDeviceConfiguration(attachment)
	objc.SetFinalizer(config, func(self *VirtioNetworkDeviceConfiguration) {
		objc.Release(self)
	})
	return config, nil
}

func newVirtioNetworkDeviceConfiguration(attachment NetworkDeviceAttachment) *VirtioNetworkDeviceConfiguration {
	ptr := C.newVZVirtioNetworkDeviceConfiguration(
		objc.Ptr(attachment),
	)
	return &VirtioNetworkDeviceConfiguration{
		pointer:    objc.NewPointer(ptr),
		attachment: attachment,
	}
}

func (v *VirtioNetworkDeviceConfiguration) SetMACAddress(macAddress *MACAddress) {
	C.setNetworkDevicesVZMACAddress(objc.Ptr(v), objc.Ptr(macAddress))
}

func (v *VirtioNetworkDeviceConfiguration) Attachment() NetworkDeviceAttachment {
	return v.attachment
}

// MACAddress represents a media access control address (MAC address), the 48-bit ethernet address.
// see: https://developer.apple.com/documentation/virtualization/vzmacaddress?language=objc
type MACAddress struct {
	*pointer
}

// NewMACAddress creates a new MACAddress with net.HardwareAddr (MAC address).
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewMACAddress(macAddr net.HardwareAddr) (*MACAddress, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	macAddrChar := charWithGoString(macAddr.String())
	defer macAddrChar.Free()
	ma := &MACAddress{
		pointer: objc.NewPointer(
			C.newVZMACAddress(macAddrChar.CString()),
		),
	}
	objc.SetFinalizer(ma, func(self *MACAddress) {
		objc.Release(self)
	})
	return ma, nil
}

// NewRandomLocallyAdministeredMACAddress creates a valid, random, unicast, locally administered address.
//
// This is only supported on macOS 11 and newer, error will
// be returned on older versions.
func NewRandomLocallyAdministeredMACAddress() (*MACAddress, error) {
	if err := macOSAvailable(11); err != nil {
		return nil, err
	}

	ma := &MACAddress{
		pointer: objc.NewPointer(
			C.newRandomLocallyAdministeredVZMACAddress(),
		),
	}
	objc.SetFinalizer(ma, func(self *MACAddress) {
		objc.Release(self)
	})
	return ma, nil
}

func (m *MACAddress) String() string {
	cstring := (*char)(C.getVZMACAddressString(objc.Ptr(m)))
	return cstring.String()
}

func (m *MACAddress) HardwareAddr() net.HardwareAddr {
	hw, _ := net.ParseMAC(m.String())
	return hw
}
