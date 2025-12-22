// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vmnet

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"text/template"
	"time"

	vzvmnet "github.com/Code-Hex/vz/v3/vmnet"
	"github.com/Code-Hex/vz/v3/xpc"
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/debugutil"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/vmnet/csops"
	"github.com/lima-vm/lima/v2/pkg/vmnet/networkchange"
)

// MARK: - Launchd Mach Service

//go:embed io.lima-vm.vmnet.plist
var launchdTemplate string

const (
	launchdLabel    = "io.lima-vm.vmnet"
	MachServiceName = launchdLabel
)

// RegisterMachService registers the "io.lima-vm.vmnet" launchd service.
//
//   - It creates a launchd plist under ~/Library/LaunchAgents and bootstraps it.
//   - The mach service "io.lima-vm.vmnet" is registered.
//   - The working directory is $LIMA_HOME/_networks/vmnet.
//   - It also creates a shell script named "io.lima-vm.vmnet.sh" that runs
//     "limactl vmnet" to avoid launching "limactl" directly from launchd.
//     macOS System Settings (General > Login Items & Extensions) shows the first
//     element of ProgramArguments as the login item name; using a shell script with
//     a fixed filename makes the item easier to identify.
func RegisterMachService(ctx context.Context) error {
	executablePath, workDir, scriptPath, launchdPlistPath, err := relatedPaths(launchdLabel)
	if err != nil {
		return err
	}
	// Check already registered
	if _, err := os.Stat(launchdPlistPath); err == nil {
		if _, err := os.Stat(scriptPath); err == nil {
			// Both files exist; assume already registered
			return nil
		}
	}

	// Create a shell script that runs "limactl vmnet"
	debugArg := ""
	if debugutil.Debug {
		debugArg = "--debug "
	}
	scriptContent := "#!/bin/sh\ntest -x " + executablePath + " && exec " + executablePath + " vmnet " + debugArg + "--mach-service='" + MachServiceName + "' \"$@\""
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		return fmt.Errorf("failed to write %q launch script: %w", scriptPath, err)
	}

	// Create launchd plist
	params := struct {
		Label            string
		ProgramArguments []string
		WorkingDirectory string
		MachServices     []string
	}{
		Label:            launchdLabel,
		ProgramArguments: []string{scriptPath},
		WorkingDirectory: workDir,
		MachServices:     []string{MachServiceName},
	}
	template, err := template.New("plist").Parse(launchdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse launchd plist template: %w", err)
	}
	var b bytes.Buffer
	if err := template.Execute(&b, params); err != nil {
		return fmt.Errorf("failed to execute launchd plist template: %w", err)
	}
	if err := os.WriteFile(launchdPlistPath, b.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write launchd plist %q: %w", launchdPlistPath, err)
	}

	// Bootstrap launchd plist
	cmd := exec.CommandContext(ctx, "launchctl", "bootstrap", launchdServiceDomain(), launchdPlistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute bootstrap: %v: %w", cmd.Args, err)
	}
	return nil
}

// UnregisterMachService unregisters the "io.lima-vm.vmnet" launchd service.
//
//   - It unbootstraps the launchd plist.
//   - It removes the launchd plist file under ~/Library/LaunchAgents.
//   - It removes the shell script used to launch "limactl vmnet".
func UnregisterMachService(ctx context.Context) error {
	serviceTarget := launchdServiceTarget(launchdLabel)
	cmd := exec.CommandContext(ctx, "launchctl", "bootout", serviceTarget)
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Infof("[vmnet] failed to execute bootout: %v", cmd.Args)
	}
	_, _, scriptPath, launchdPlistPath, err := relatedPaths(launchdLabel)
	if err != nil {
		return err
	}
	if err := os.Remove(launchdPlistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove launchd plist %q: %w", launchdPlistPath, err)
	}
	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove launch script file %q: %w", scriptPath, err)
	}
	return nil
}

func relatedPaths(launchdLabel string) (executablePath, workDir, scriptPath, plistPath string, err error) {
	executablePath, err = os.Executable()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get executable path: %w", err)
	}
	networksDir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get Lima networks directory: %w", err)
	}
	// Working directory
	workDir = filepath.Join(networksDir, "vmnet")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", "", "", "", fmt.Errorf("failed to create working directory %q: %w", workDir, err)
	}
	// Shell script path
	scriptPath = filepath.Join(workDir, launchdLabel+".sh")
	// Launchd plist path
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	plistPath = filepath.Join(userHomeDir, "Library", "LaunchAgents", launchdLabel+".plist")
	return executablePath, workDir, scriptPath, plistPath, nil
}

func launchdServiceDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func launchdServiceTarget(launchdLabel string) string {
	return fmt.Sprintf("%s/%s", launchdServiceDomain(), launchdLabel)
}

// RunMachService runs the mach service at specified service name.
//
// It listens for incoming mach messages requesting a VmnetNetwork
// for a given network, creates the VmnetNetwork if not already created,
// and returns the serialized network object via mach XPC.
func RunMachService(ctx context.Context, serviceName string) (err error) {
	// Create peer requirement to restrict clients to the same executable.
	var peerRequirement *xpc.PeerRequirement
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		return err
	}
	// Until macOS 26.1, VZVmnetNetworkDeviceAttachment could not connect to vmnet networks created by different executables.
	// From macOS 26.2, this restriction seems lifted.
	// Check the OS version to decide whether to set the peer requirement.
	if macOSProductVersion.LessThan(*semver.New("26.2.0")) {
		peerRequirement, err = peerRequirementForRestrictToSameExecutable()
		if err != nil {
			return fmt.Errorf("failed to create peer requirement: %w", err)
		}
	}
	networkEntries := make(map[string]*Entry)
	var mu sync.RWMutex
	var adaptorWG sync.WaitGroup
	defer adaptorWG.Wait() // Wait for all adaptors to finish
	listener, err := xpc.NewListener(serviceName,
		xpc.Accept(
			xpc.MessageHandler(func(dic *xpc.Dictionary) *xpc.Dictionary {
				errorReply := func(errMsg string, args ...any) *xpc.Dictionary {
					logrus.Errorf("[vmnet] "+errMsg, args...)
					return dic.CreateReply(
						xpc.KeyValue("Error", xpc.NewString(fmt.Sprintf(errMsg, args...))),
					)
				}

				// Verify peer requirement on macOS 26.0 and 26.1
				if peerRequirement != nil {
					// If client did not specify FdType, they are likely using VZVmnetNetworkDeviceAttachment.
					// In that case, enforce the peer requirement.
					if fdType := dic.GetString("FdType"); fdType == "" {
						// Verify that the sender satisfies the peer requirement.
						// This ensures that only clients from the same executable can request networks.
						// This is necessary because vzvmnet.Network cannot be shared across different executables.
						// The requests from external VZ drivers will be rejected here.
						if ok, err := dic.SenderSatisfies(peerRequirement); err != nil {
							return errorReply("failed to verify sender requirement: %v", err)
						} else if !ok {
							return errorReply("sender does not satisfy peer requirement")
						}
					}
				}

				// Handle the message
				vmnetNetwork := dic.GetString("Network")
				if vmnetNetwork == "" {
					return errorReply("missing Network key")
				}
				// Check if the network is already registered
				mu.RLock()
				entry, ok := networkEntries[vmnetNetwork]
				mu.RUnlock()
				if ok {
					mu.Lock()
					entry.lastRequestAt = time.Now()
					mu.Unlock()
				} else {
					entry, err = newEntry(dic)
					if err != nil {
						return errorReply("failed to create Entry for 'vmnet: %s': %v", vmnetNetwork, err)
					}
					mu.Lock()
					networkEntries[vmnetNetwork] = entry
					mu.Unlock()
					logrus.Infof("[vmnet] created new subnet %v for 'vmnet: %q'", entry.config.Subnet, vmnetNetwork)
				}
				replyEntries := slices.Clone(entry.replyEntries)

				// If the FdType is specified in the message, create file adaptor and include it's file descriptor in the reply.
				file, startAdaptor, err := newFileAdaptorForNetwork(ctx, entry.network, dic)
				if err != nil {
					return errorReply("failed to create file adaptor for 'vmnet: %s': %v", vmnetNetwork, err)
				} else if file != nil {
					replyEntries = append(replyEntries, xpc.KeyValue("FileDescriptor", xpc.NewFd(file.Fd())))
					_ = file.Close() // The file descriptor is duplicated when creating xpc.NewFd
				}

				if startAdaptor != nil {
					adaptorWG.Add(1)
					go func() {
						defer adaptorWG.Done()
						startAdaptor()
					}()
				}

				return dic.CreateReply(replyEntries...)
			}),
		),
	)
	if err != nil {
		return err
	}
	defer func() {
		if closeError := listener.Close(); closeError != nil {
			if err != nil {
				err = errors.Join(err, closeError)
			} else {
				err = closeError
			}
		}
	}()
	if err := listener.Activate(); err != nil {
		return err
	}
	// Set up network change notifier to clear cached VmnetNetworks
	notifyCh := make(chan struct{}, 20)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-notifyCh:
			}

			// Handle network change notification here
			logrus.Debug("[vmnet] network change detected; clearing cached VmnetNetworks")
			ifaces, err := NewInterfaces()
			if err != nil {
				logrus.Errorf("[vmnet] failed to list interfaces on network change: %v", err)
				// Hopefully the next notification will succeed
				continue
			}
			// Remove entries whose interfaces are gone
			mu.Lock()
			for vzNetwork, entry := range networkEntries {
				if iface := ifaces.LookupInterface(entry.config.Subnet); iface != nil {
					if entry.existenceObserved {
						continue
					}
					if iface.Type == syscall.IFT_BRIDGE {
						logrus.Debugf("[vmnet] interface for subnet %v of 'vmnet: %q' exists; keeping cached VmnetNetwork", entry.config.Subnet, vzNetwork)
						entry.existenceObserved = true
					} else {
						logrus.Debugf("[vmnet] interface for subnet %v of 'vmnet: %q' is found but not a bridge (type=%d); removing cached VmnetNetwork since it cannot be used", entry.config.Subnet, vzNetwork, iface.Type)
						delete(networkEntries, vzNetwork)
					}
				} else if !entry.existenceObserved {
					if time.Since(entry.lastRequestAt) < 1*time.Minute {
						logrus.Debugf("[vmnet] interface for subnet %v of 'vmnet: %q' is not found yet; keeping cached VmnetNetwork", entry.config.Subnet, vzNetwork)
					} else {
						logrus.Infof("[vmnet] interface for subnet %v of 'vmnet: %q' is not found for more than 1 minute; removing cached VmnetNetwork", entry.config.Subnet, vzNetwork)
						delete(networkEntries, vzNetwork)
					}
				} else {
					logrus.Infof("[vmnet] interface for subnet %v of 'vmnet: %q' is gone; removing cached VmnetNetwork", entry.config.Subnet, vzNetwork)
					delete(networkEntries, vzNetwork)
				}
			}
			mu.Unlock()
			if len(networkEntries) == 0 {
				logrus.Info("[vmnet] no cached VmnetNetworks remain, stopping mach service")
				cancel()
			}
		}
	}()
	notifier := networkchange.NewNotifier(func(_ *networkchange.Notifier) {
		notifyCh <- struct{}{}
	})
	defer notifier.Cancel()
	<-ctx.Done()
	return nil
}

// peerRequirementForRestrictToSameExecutable creates a [xpc.PeerRequirement]
// that restricts clients to the same executable by CDHash.
func peerRequirementForRestrictToSameExecutable() (*xpc.PeerRequirement, error) {
	cdhash, err := csops.SelfCdhash()
	if err != nil {
		return nil, fmt.Errorf("failed to get self CDHash: %w", err)
	}
	peerRequirement, err := xpc.NewPeerRequirementLwcrWithEntries(xpc.KeyValue("cdhash", xpc.NewData(cdhash)))
	if err != nil {
		return nil, fmt.Errorf("failed to create peer requirement: %w", err)
	}
	return peerRequirement, nil
}

// Entry represents a cached VmnetNetwork entry.
type Entry struct {
	config            *networks.VmnetConfig
	network           *vzvmnet.Network
	replyEntries      []xpc.DictionaryEntry
	existenceObserved bool
	lastRequestAt     time.Time
}

// newEntry creates a new Entry from the given xpc.Dictionary.
func newEntry(dic *xpc.Dictionary) (*Entry, error) {
	// The Configuration key must be provided in the message to create the VmnetNetwork.
	var vmnetConfig networks.VmnetConfig
	var vmnetNetwork *vzvmnet.Network
	var serialization xpc.Object
	config := dic.GetData("Configuration")
	if config == nil {
		return nil, errors.New("missing Configuration key")
	} else if err := json.Unmarshal(config, &vmnetConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VzVmnetConfig: %w", err)
	} else if vmnetNetwork, err = newVmnetNetwork(vmnetConfig); err != nil {
		return nil, err
	} else if serialization, err = vmnetNetwork.CopySerialization(); err != nil {
		return nil, err
	}
	// If the Subnet is not set in the config, retrieve it from the created VmnetNetwork.
	// This ensures that the Subnet is always set in the Entry.
	if !vmnetConfig.Subnet.IsValid() {
		subnet, err := vmnetNetwork.IPv4Subnet()
		if err != nil {
			return nil, fmt.Errorf("failed to get IPv4 subnet from VmnetNetwork: %w", err)
		}
		vmnetConfig.Subnet = subnet
	}
	return &Entry{
		config:  &vmnetConfig,
		network: vmnetNetwork,
		replyEntries: []xpc.DictionaryEntry{
			xpc.KeyValue("Configuration", xpc.NewData(config)),
			xpc.KeyValue("Serialization", serialization),
		},
		lastRequestAt: time.Now(),
	}, nil
}

// newVmnetNetwork creates a new [vzvmnet.Network] for the given [networks.VmnetConfig].
func newVmnetNetwork(vmnetConfig networks.VmnetConfig) (*vzvmnet.Network, error) {
	var vmnetMode vzvmnet.Mode
	switch vmnetConfig.Mode {
	case networks.VmnetModeShared:
		vmnetMode = vzvmnet.SharedMode
	case networks.VmnetModeHost:
		vmnetMode = vzvmnet.HostMode
	default:
		return nil, fmt.Errorf("unknown VzVmnetMode: %q", vmnetConfig.Mode)
	}
	config, err := vzvmnet.NewNetworkConfiguration(vmnetMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create network configuration with mode: %q: %w", vmnetMode, err)
	}
	if !*vmnetConfig.Dhcp {
		config.DisableDhcp()
	}
	if !*vmnetConfig.DNSProxy {
		config.DisableDnsProxy()
	}
	if vmnetConfig.Mtu != 0 {
		if err := config.SetMtu(vmnetConfig.Mtu); err != nil {
			return nil, fmt.Errorf("failed to set MTU to %d: %w", vmnetConfig.Mtu, err)
		}
	}
	if !*vmnetConfig.Nat44 {
		config.DisableNat44()
	}
	if !*vmnetConfig.Nat66 {
		config.DisableNat66()
	}
	if !*vmnetConfig.RouterAdvertisement {
		config.DisableRouterAdvertisement()
	}
	if vmnetConfig.Subnet.IsValid() {
		if err := config.SetIPv4Subnet(vmnetConfig.Subnet); err != nil {
			return nil, fmt.Errorf("failed to set IPv4 subnet to %s: %w", vmnetConfig.Subnet, err)
		}
	}
	return vzvmnet.NewNetwork(config)
}

// newFileAdaptorForNetwork creates a file adaptor for the created interface of the network cloned from the given network.
// The file adaptor type is determined by the "FdType" key in the given xpc.Dictionary.
// If no FdType is specified, nil is returned without error.
// The returned file can be used as a file descriptor for QEMU's netdev or krunkit's virtio-net.
func newFileAdaptorForNetwork(ctx context.Context, network *vzvmnet.Network, dic *xpc.Dictionary) (*os.File, func(), error) {
	fdType := dic.GetString("FdType")
	if fdType == "" {
		return nil, nil, nil
	}
	interfaceDesc := dic.GetDictionary("InterfaceDesc")
	var fileAdaptorFunc func(context.Context, *vzvmnet.Interface, ...vzvmnet.Sockopt) (*os.File, func(), error)
	switch FdType(fdType) {
	case FdTypeStream:
		fileAdaptorFunc = streamFileAdaptorForInterface
	case FdTypeDatagram:
		fileAdaptorFunc = datagramFileAdaptorForInterface
	case FdTypeDatagramNext:
		fileAdaptorFunc = datagramNextFileAdaptorForInterface
	default:
		return nil, nil, fmt.Errorf("unknown FdType: %q", fdType)
	}
	subnet, err := network.IPv4Subnet()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get IPv4 subnet from interface: %w", err)
	}
	// Create New Network from serialization to avoid releasing the original network on stopping the interface.
	var iface *vzvmnet.Interface
	if s, err := network.CopySerialization(); err != nil {
		return nil, nil, fmt.Errorf("failed to copy network serialization: %w", err)
	} else if n, err := vzvmnet.NewNetworkWithSerialization(s); err != nil {
		return nil, nil, fmt.Errorf("failed to create network from serialization %+v: %w", s, err)
	} else if iface, err = vzvmnet.StartInterfaceWithNetwork(n, interfaceDesc); err != nil {
		return nil, nil, fmt.Errorf("failed to start interface with network %+v: %w", n, err)
	}
	logrus.Infof("[vmnet] created %s file adaptor for subnet: %v", fdType, subnet)
	logrus.Debugf("[vmnet] started vmnet.Interface with maxPacketSize=%d, maxReadPacketCount=%d, maxWritePacketCount=%d, param=%+v",
		iface.MaxPacketSize, iface.MaxReadPacketCount, iface.MaxWritePacketCount, iface.Param)
	return fileAdaptorFunc(ctx, iface)
}

// MARK: - Request Network

// RequestNetwork requests the [vzvmnet.Network] for the given vmnetNetwork name.
func RequestNetwork(ctx context.Context, vmnetNetwork string) (*vzvmnet.Network, error) {
	reply, err := RequestNetworkWithEntries(ctx, vmnetNetwork)
	if err != nil {
		return nil, err
	}
	// Extract Serialization from reply
	serialization := reply.GetValue("Serialization")
	if serialization == nil {
		return nil, fmt.Errorf("no Serialization object in reply from %q", MachServiceName)
	}
	network, err := vzvmnet.NewNetworkWithSerialization(serialization)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'vmnet: %s' from serialization: %w", vmnetNetwork, err)
	}
	return network, nil
}

// MARK: - Request File Descriptors

// RequestQEMUDatagramFileDescriptorForNetwork requests a datagram file descriptor for the given vmnetNetwork name.
// This can be used with QEMU's netdev of type "datagram" or "tap".
func RequestQEMUDatagramFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	// Use DatagramNext for better performance
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeDatagram,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			// QEMU does not support TSO, but TSO disabled interfaces cause performance regression on another interfaces.
			// So, enable TSO here to avoid performance regression on other interfaces.
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestQEMUDatagramNextFileDescriptorForNetwork requests a datagram file descriptor for the given vmnetNetwork name.
// This can be used with QEMU's netdev of type "datagram" or "tap".
func RequestQEMUDatagramNextFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	// Use DatagramNext for better performance
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeDatagramNext,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			// QEMU does not support TSO, but TSO disabled interfaces cause performance regression on another interfaces.
			// So, enable TSO here to avoid performance regression on other interfaces.
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestQEMUStreamFileDescriptorForNetwork requests a stream file descriptor for the given vmnetNetwork name.
// This can be used with QEMU's netdev of type "socket" or "stream".
func RequestQEMUStreamFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeStream,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			// QEMU does not support TSO, but TSO disabled interfaces cause performance regression on another interfaces.
			// So, enable TSO here to avoid performance regression on other interfaces.
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestKrunkitDatagramFileDescriptorForNetwork requests a datagram file descriptor for the given vmnetNetwork name.
// This can be used with Krunkit's virtio-net device as a unix datagram socket.
// Enabled checksum offload and TSO for krunkit, as krunkit can handle them.
// Use "offloading=on" in the virtio-net device options to enable them in krunkit.
func RequestKrunkitDatagramFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeDatagram,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestKrunkitDatagramNextFileDescriptorForNetwork requests a datagram_next file descriptor for the given vmnetNetwork name.
// This can be used with Krunkit's virtio-net device as a unix datagram socket.
// Enabled checksum offload and TSO for krunkit, as krunkit can handle them.
// Use "offloading=on" in the virtio-net device options to enable them in krunkit.
func RequestKrunkitDatagramNextFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeDatagramNext,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestKrunkitStreamFileDescriptorForNetwork requests a stream file descriptor for the given vmnetNetwork name.
// This can be used with Krunkit's virtio-net device as a unix stream socket.
// Enabled checksum offload and TSO for krunkit, as krunkit can handle them.
// Use "offloading=on" in the virtio-net device options to enable them in krunkit.
func RequestKrunkitStreamFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeStream,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// RequestVZDatagramFileDescriptorForNetwork requests a datagram file descriptor for the given vmnetNetwork name.
// This can be used with [vz.NewFileHandleNetworkDeviceAttachment].
// This API is used when VZ is external driver and macOS version is 26.1 or earlier,
// as VZVmnetNetworkDeviceAttachment cannot connect to vmnet networks created by different executables on those macOS versions.
func RequestVZDatagramFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string) (*os.File, error) {
	return RequestFileDescriptorForNetwork(ctx, vmnetNetwork, FdTypeDatagramNext,
		xpc.KeyValue("InterfaceDesc", xpc.NewDictionary(
			xpc.KeyValue(vzvmnet.EnableChecksumOffloadKey, xpc.BoolTrue),
			// VZVmnetNetworkDeviceAttachment does not support TSO, but TSO disabled interfaces cause performance regression on another interfaces.
			// So, enable TSO here to avoid performance regression on other interfaces.
			// By enabling TSO here, performance of this interface becomes much worse.
			xpc.KeyValue(vzvmnet.EnableTSOKey, xpc.BoolTrue),
		)),
	)
}

// FdType represents the type of file descriptor to request.
type FdType string

const (
	FdTypeStream       FdType = "stream"
	FdTypeDatagram     FdType = "datagram"
	FdTypeDatagramNext FdType = "datagram_next"
)

// RequestFileDescriptorForNetwork requests the file descriptor for the given vmnetNetwork name.
func RequestFileDescriptorForNetwork(ctx context.Context, vmnetNetwork string, fdType FdType, entries ...xpc.DictionaryEntry) (*os.File, error) {
	reply, err := RequestNetworkWithEntries(ctx,
		vmnetNetwork,
		slices.Concat([]xpc.DictionaryEntry{
			xpc.KeyValue("FdType", xpc.NewString(string(fdType))),
		}, entries)...,
	)
	if err != nil {
		return nil, err
	}
	// Extract FileDescriptor from reply
	fd := reply.DupFd("FileDescriptor")
	if fd <= 0 {
		return nil, fmt.Errorf("no FileDescriptor object in reply from %q", MachServiceName)
	}

	name := fmt.Sprintf("vmnet:%s:%s:%d", vmnetNetwork, fdType, fd)
	if v, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF); err != nil {
		logrus.WithError(err).Debugf("[vmnet] failed to get SO_RCVBUF for %s", name)
	} else {
		logrus.Debugf("[vmnet] got SO_RCVBUF=%d for %s", v, name)
	}
	if v, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF); err != nil {
		logrus.WithError(err).Debugf("[vmnet] failed to get SO_SNDBUF for %s", name)
	} else {
		logrus.Debugf("[vmnet] got SO_SNDBUF=%d for %s", v, name)
	}
	if v, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVLOWAT); err != nil {
		logrus.WithError(err).Debugf("[vmnet] failed to get SO_RCVLOWAT for %s", name)
	} else {
		logrus.Debugf("[vmnet] got SO_RCVLOWAT=%d for %s", v, name)
	}
	if v, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDLOWAT); err != nil {
		logrus.WithError(err).Debugf("[vmnet] failed to get SO_SNDLOWAT for %s", name)
	} else {
		logrus.Debugf("[vmnet] got SO_SNDLOWAT=%d for %s", v, name)
	}
	file := os.NewFile(fd, name)
	return file, nil
}

// MARK: - Base Request Function

// RequestNetworkWithEntries requests the [vzvmnet.Network] entry for the given vmnetNetwork
//
// Payload to the mach service:
//
//	{`Network`: <vmnetNetwork>, `Configuration`: <configuration>, ...entries}
//
// Reply from the mach service:
//
//	{`Configuration`: <configuration>, (`Serialization`: <serialization> | `FileDescriptor`: <file descriptor>) }
//
// If an error occurs, the reply contains:
//
//	{`Error`: <error message>}
func RequestNetworkWithEntries(ctx context.Context, vmnetNetwork string, entries ...xpc.DictionaryEntry) (*xpc.Dictionary, error) {
	// Load network configuration
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return nil, err
	}
	vmnetConfig, ok := nwCfg.Vmnet[vmnetNetwork]
	if !ok {
		return nil, fmt.Errorf("networks.yaml: 'vmnet: %s' is not defined", vmnetNetwork)
	}
	ourConfigBytes, err := json.Marshal(vmnetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal our 'vmnet: %s' config: %w", vmnetNetwork, err)
	}

	session, err := xpc.NewSession(MachServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create xpc session to %q: %w", MachServiceName, err)
	}
	defer session.Cancel()
	request := slices.Concat([]xpc.DictionaryEntry{
		xpc.KeyValue("Network", xpc.NewString(vmnetNetwork)),
		xpc.KeyValue("Configuration", xpc.NewData(ourConfigBytes)),
	}, entries)
	reply, err := session.SendDictionaryWithReply(ctx, request...)
	if err != nil {
		return nil, fmt.Errorf("failed to send xpc message to %q: %w", MachServiceName, err)
	}
	// Check for error in reply
	if errMsg := reply.GetString("Error"); errMsg != "" {
		return nil, fmt.Errorf("error from mach service %q: %s", MachServiceName, errMsg)
	}

	// Check that the configuration matches our expected configuration.
	// Warn if it does not match.
	providedConfigBytes := reply.GetData("Configuration")
	if providedConfigBytes == nil {
		return nil, fmt.Errorf("no Configuration object in reply from %q", MachServiceName)
	}
	var providedConfig networks.VmnetConfig
	if err := json.Unmarshal(providedConfigBytes, &providedConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provided 'vmnet: %s' config: %w", vmnetNetwork, err)
	}

	// If the Subnet is not set in our config, the provided config will have it set.
	if !vmnetConfig.Subnet.IsValid() {
		vmnetConfig.Subnet = providedConfig.Subnet
	}

	// Warn if the provided configuration does not match our expected configuration.
	if !reflect.DeepEqual(providedConfig, vmnetConfig) {
		logrus.Warnf("[vmnet] existing 'vmnet: %s' has different configuration; our config: %v, existing config: %v", vmnetNetwork, vmnetConfig, providedConfig)
	}
	return reply, nil
}
