// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vzvmnet

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"text/template"
	"time"
	"unsafe"

	"github.com/Code-Hex/vz/v3/pkg/vmnet"
	"github.com/Code-Hex/vz/v3/pkg/xpc"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/vzvmnet/csops"
	"github.com/lima-vm/lima/v2/pkg/vzvmnet/networkchange"
)

//go:embed io.lima-vm.vz.vmnet.plist
var launchdTemplate string

const (
	launchdLabel    = "io.lima-vm.vz.vmnet"
	MachServiceName = launchdLabel
)

// RegisterMachService registers the "io.lima-vm.vz.vmnet" launchd service.
//
//   - It creates a launchd plist under ~/Library/LaunchAgents and bootstraps it.
//   - The mach service "io.lima-vm.vz.vmnet" is registered.
//   - The working directory is $LIMA_HOME/_networks/vz-vmnet.
//   - It also creates a shell script named "io.lima-vm.vz.vmnet.sh" that runs
//     "limactl vz-vmnet" to avoid launching "limactl" directly from launchd.
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

	// Create a shell script that runs "limactl vz-vmnet"
	scriptContent := "#!/bin/sh\ntest -x " + executablePath + " && exec " + executablePath + " vz-vmnet --mach-service='" + MachServiceName + "' \"$@\""
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

// UnregisterMachService unregisters the "io.lima-vm.vz.vmnet" launchd service.
//
//   - It unbootstraps the launchd plist.
//   - It removes the launchd plist file under ~/Library/LaunchAgents.
//   - It removes the shell script used to launch "limactl vz-vmnet".
func UnregisterMachService(ctx context.Context) error {
	serviceTarget := launchdServiceTarget(launchdLabel)
	cmd := exec.CommandContext(ctx, "launchctl", "bootout", serviceTarget)
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Infof("failed to execute bootout: %v", cmd.Args)
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
	workDir = filepath.Join(networksDir, "vz-vmnet")
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
// for a given vz network, creates the VmnetNetwork if not already created,
// and returns the serialized network object via mach XPC.
func RunMachService(ctx context.Context, serviceName string) (err error) {
	// Create peer requirement to restrict clients to the same executable.
	peerRequirement, err := peerRequirementForRestrictToSameExecutable()
	if err != nil {
		return fmt.Errorf("failed to create peer requirement: %w", err)
	}
	networkEntries := make(map[string]*Entry)
	var mu sync.RWMutex
	listener, err := xpc.NewListener(serviceName,
		xpc.Accept(
			xpc.MessageHandler(func(dic *xpc.Dictionary) *xpc.Dictionary {
				errorReply := func(errMsg string, args ...any) *xpc.Dictionary {
					return dic.CreateReply(
						xpc.KeyValue("Error", xpc.NewString(fmt.Sprintf(errMsg, args...))),
					)
				}

				// Verify that the sender satisfies the peer requirement.
				// This ensures that only clients from the same executable can request networks.
				// This is necessary because VZVmnetNetwork cannot be shared across different executables.
				// The requests from external VZ drivers will be rejected here.
				if ok, err := dic.SenderSatisfies(peerRequirement); err != nil {
					return errorReply("failed to verify sender requirement: %v", err)
				} else if !ok {
					return errorReply("sender does not satisfy peer requirement")
				}

				// Handle the message
				vzNetwork := dic.GetString("Network")
				if vzNetwork == "" {
					return errorReply("missing Network key")
				}
				// Check if the network is already registered
				mu.RLock()
				entry, ok := networkEntries[vzNetwork]
				mu.RUnlock()
				if ok {
					logrus.Infof("Provided existing VmnetNetwork for 'vz: %q'", vzNetwork)
					return dic.CreateReply(entry.replyEntries...)
				}

				logrus.Infof("No existing VmnetNetwork for 'vz: %q'", vzNetwork)
				entry, err := newEntry(dic)
				if err != nil {
					return errorReply("failed to create Entry for 'vz: %s': %v", vzNetwork, err)
				}
				mu.Lock()
				networkEntries[vzNetwork] = entry
				mu.Unlock()
				logrus.Infof("Created new VmnetNetwork for 'vz: %q'", vzNetwork)
				return dic.CreateReply(entry.replyEntries...)
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
		// Use a timer to avoid flooding logs on rapid network changes since
		// multiple notifications may be received on a VM start or stop.
		const distantFutureDuration time.Duration = math.MaxInt64
		const timeoutToNextNotification time.Duration = 3 * time.Second
		timer := time.NewTimer(distantFutureDuration)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-notifyCh:
				// Avoid flooding logs by resetting the timer to timeoutToNextNotification
				timer.Reset(timeoutToNextNotification)
				continue
			case <-timer.C:
				// Reset the timer to distantFutureDuration
				timer.Reset(distantFutureDuration)
			}

			// Handle network change notification here
			logrus.Info("Network change detected; clearing cached VmnetNetworks")
			ifaces, err := NewInterfaces()
			if err != nil {
				logrus.Errorf("Failed to list interfaces on network change: %v", err)
				// Hopefully the next notification will succeed
				continue
			}
			// Remove entries whose interfaces are gone
			mu.Lock()
			for vzNetwork, entry := range networkEntries {
				if iface := ifaces.LookupInterface(entry.config.Subnet); iface != nil {
					if iface.Type == syscall.IFT_BRIDGE {
						logrus.Infof("Interface for subnet %v of 'vz: %q' exists; keeping cached VmnetNetwork", entry.config.Subnet, vzNetwork)
						entry.existenceObserved = true
					} else {
						logrus.Infof("Interface for subnet %v of 'vz: %q' is found but not a bridge (type=%d); removing cached VmnetNetwork since it cannot be used", entry.config.Subnet, vzNetwork, iface.Type)
						delete(networkEntries, vzNetwork)
					}
				} else if !entry.existenceObserved {
					logrus.Infof("Interface for subnet %v of 'vz: %q' is not found yet; keeping cached VmnetNetwork", entry.config.Subnet, vzNetwork)
				} else {
					logrus.Infof("Interface for subnet %v of 'vz: %q' is gone; removing cached VmnetNetwork", entry.config.Subnet, vzNetwork)
					delete(networkEntries, vzNetwork)
				}
			}
			mu.Unlock()
			if len(networkEntries) == 0 {
				logrus.Info("No cached VmnetNetworks remain, stopping mach service")
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
	config            *networks.VzVmnetConfig
	network           *vmnet.Network
	replyEntries      []xpc.DictionaryEntry
	existenceObserved bool
}

// newEntry creates a new Entry from the given xpc.Dictionary.
func newEntry(dic *xpc.Dictionary) (*Entry, error) {
	// The Configuration key must be provided in the message to create the VmnetNetwork.
	var vmnetConfig networks.VzVmnetConfig
	var vmnetNetwork *vmnet.Network
	var serialization unsafe.Pointer
	config := dic.GetData("Configuration")
	if config == nil {
		return nil, errors.New("missing Configuration key")
	} else if err := json.Unmarshal(config, &vmnetConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal VzVmnetConfig: %w", err)
	} else if vmnetNetwork, err = newVmnetNetwork(vmnetConfig); err != nil {
		return nil, fmt.Errorf("failed to create VmnetNetwork: %w", err)
	} else if serialization, err = vmnetNetwork.CopySerialization(); err != nil {
		return nil, fmt.Errorf("failed to copy VmnetNetwork serialization: %w", err)
	}
	return &Entry{
		config:  &vmnetConfig,
		network: vmnetNetwork,
		replyEntries: []xpc.DictionaryEntry{
			xpc.KeyValue("Configuration", xpc.NewData(config)),
			xpc.KeyValue("Serialization", xpc.NewObject(serialization)),
		},
	}, nil
}

// newVmnetNetwork creates a new [vz.VmnetNetwork] for the given [networks.VzVmnetConfig].
func newVmnetNetwork(vmnetConfig networks.VzVmnetConfig) (*vmnet.Network, error) {
	var vmnetMode vmnet.Mode
	switch vmnetConfig.Mode {
	case networks.VzModeShared:
		vmnetMode = vmnet.SharedMode
	case networks.VzModeHost:
		vmnetMode = vmnet.HostMode
	default:
		return nil, fmt.Errorf("unknown VzVmnetMode: %q", vmnetConfig.Mode)
	}
	config, err := vmnet.NewNetworkConfiguration(vmnetMode)
	if err != nil {
		return nil, fmt.Errorf("failed to create network configuration with mode: %q: %w", vmnetMode, err)
	}
	if !vmnetConfig.Dhcp {
		config.DisableDhcp()
	}
	if !vmnetConfig.DNSProxy {
		config.DisableDnsProxy()
	}
	if vmnetConfig.Mtu != 0 {
		if err := config.SetMtu(vmnetConfig.Mtu); err != nil {
			return nil, fmt.Errorf("failed to set MTU to %d: %w", vmnetConfig.Mtu, err)
		}
	}
	if !vmnetConfig.Nat44 {
		config.DisableNat44()
	}
	if !vmnetConfig.Nat66 {
		config.DisableNat66()
	}
	if !vmnetConfig.RouterAdvertisement {
		config.DisableRouterAdvertisement()
	}
	if vmnetConfig.Subnet.IsValid() {
		if err := config.SetIPv4Subnet(vmnetConfig.Subnet); err != nil {
			return nil, fmt.Errorf("failed to set IPv4 subnet to %s: %w", vmnetConfig.Subnet, err)
		}
	}

	network, err := vmnet.NewNetwork(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create VmnetNetwork: %w", err)
	}
	return network, nil
}

// RequestVmnetNetwork requests the [vz.VmnetNetwork] serialization
// for the given vzNetwork from the mach service "io.lima-vm.vz.vmnet.subnet".
//
// Payload to the mach service:
//
//	{`Network`: <vzNetwork>, `Configuration`: <configuration>}
//
// Reply from the mach service:
//
//	{`Configuration`: <configuration>, `Serialization`: <serialization>}
//
// If an error occurs, the reply contains:
//
//	{`Error`: <error message>}
func RequestVmnetNetwork(ctx context.Context, vzNetwork string, vmnetConfig networks.VzVmnetConfig) (*vmnet.Network, error) {
	// Ensure that the mach service is registered
	if err := RegisterMachService(ctx); err != nil {
		return nil, err
	}

	ourConfig, err := json.Marshal(vmnetConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal our 'vz: %s' config: %w", vzNetwork, err)
	}

	session, err := xpc.NewSession(MachServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create xpc session to %q: %w", MachServiceName, err)
	}
	defer session.Cancel()
	reply, err := session.SendDictionaryWithReply(
		ctx,
		xpc.KeyValue("Network", xpc.NewString(vzNetwork)),
		xpc.KeyValue("Configuration", xpc.NewData(ourConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send xpc message to %q: %w", MachServiceName, err)
	}
	// Check for error in reply
	if errMsg := reply.GetString("Error"); errMsg != "" {
		return nil, fmt.Errorf("error from mach service %q: %s", MachServiceName, errMsg)
	}

	// Check that the configuration matches our expected configuration.
	// Warn if it does not match.
	config := reply.GetData("Configuration")
	if config == nil {
		return nil, fmt.Errorf("no Configuration object in reply from %q", MachServiceName)
	}
	if !slices.Equal(config, ourConfig) {
		logrus.Warnf("Existing 'vz: %s' has different configuration; our config: %s, existing config: %s", vzNetwork, string(ourConfig), string(config))
	}

	serialization := reply.GetValue("Serialization")
	if serialization == nil {
		return nil, fmt.Errorf("no Serialization object in reply from %q", MachServiceName)
	}
	network, err := vmnet.NewNetworkWithSerialization(serialization.Raw())
	if err != nil {
		return nil, fmt.Errorf("failed to create 'vz: %s' from serialization: %w", vzNetwork, err)
	}
	return network, nil
}
