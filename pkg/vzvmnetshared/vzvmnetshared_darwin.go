package vzvmnetshared

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/Code-Hex/vz/v3"
	"github.com/Code-Hex/vz/v3/pkg/xpc"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
)

//go:embed io.lima-vm.vz.vmnet.shared.plist
var launchdTemplate string

const (
	launchdLabel    = "io.lima-vm.vz.vmnet.shared"
	MachServiceName = launchdLabel + ".subnet"
)

// RegisterMachService registers the "io.lima-vm.vz.vmnet.shared" launchd service.
//
//   - It creates a launchd plist under ~/Library/LaunchAgents and bootstraps it.
//   - The mach service "io.lima-vm.vz.vmnet.shared.subnet" is registered.
//   - The working directory is $LIMA_HOME/_networks/vz-vmnet-shared.
//   - It also creates a shell script named "io.lima-vm.vz.vmnet.shared.sh" that runs
//     "limactl vz-vmnet-shared" to avoid launching "limactl" directly from launchd.
//     macOS System Settings (General > Login Items & Extensions) shows the first
//     element of ProgramArguments as the login item name; using a shell script with
//     a fixed filename makes the item easier to identify.
func RegisterMachService(ctx context.Context) error {
	executablePath, workDir, scriptPath, launchdPlistPath, err := relatedPaths(launchdLabel)
	if err != nil {
		return err
	}

	// Create a shell script that runs "limactl vz-vmnet-shared"
	scriptContent := "#!/bin/sh\nexec " + executablePath + " vz-vmnet-shared --mach-service='" + MachServiceName + "' \"$@\""
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
	cmd := exec.CommandContext(ctx, "launchctl", "bootstrap", serviceDomain(), launchdPlistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute bootstrap: %v: %w", cmd.Args, err)
	}
	return nil
}

// UnregisterMachService unregisters the "io.lima-vm.vz.vmnet.shared" launchd service.
//
//   - It unbootstraps the launchd plist.
//   - It removes the launchd plist file under ~/Library/LaunchAgents.
//   - It removes the shell script used to launch "limactl vz-vmnet-shared".
func UnregisterMachService(ctx context.Context) error {
	serviceTarget := serviceTarget(launchdLabel)
	cmd := exec.CommandContext(ctx, "launchctl", "bootout", serviceTarget)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute bootout: %v: %w", cmd.Args, err)
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
	workDir = filepath.Join(networksDir, "vz-vmnet-shared")
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

func serviceDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

func serviceTarget(launchdLabel string) string {
	return fmt.Sprintf("%s/%s", serviceDomain(), launchdLabel)
}

// RunMachService runs the mach service at specified service name.
//
// It listens for incoming mach messages requesting a shared VmnetNetwork
// for a given subnet, creates the VmnetNetwork if not already created,
// and returns the serialized network object via mach IPC.
func RunMachService(ctx context.Context, serviceName string) (err error) {
	serializationStore := make(map[netip.Prefix]*xpc.Object)
	listener, err := xpc.NewListener(serviceName,
		xpc.NewSessionHandler(
			func(msg *xpc.Object) *xpc.Object {
				errorReply := func(errMsg string, args ...any) *xpc.Object {
					return msg.DictionaryCreateReply(
						xpc.WithString("Error", fmt.Sprintf(errMsg, args...)),
					)
				}
				// Handle the message
				subnetStr := msg.DictionaryGetString("Subnet")
				if subnetStr == "" {
					return errorReply("missing Subnet key")
				}
				prefix, err := netip.ParsePrefix(subnetStr)
				if err != nil {
					return errorReply("failed to parse Subnet %q: %v", subnetStr, err)
				}
				// Modify the prefix to having IP that VmnetNetwork can accept.
				prefix = netip.PrefixFrom(prefix.Masked().Addr().Next(), prefix.Bits())
				serialization, ok := serializationStore[prefix]
				if !ok {
					if config, err := vz.NewVmnetNetworkConfiguration(vz.SharedMode); err != nil {
						return errorReply("failed to create network configuration: %v", err)
					} else if err := config.SetIPv4Subnet(prefix); err != nil {
						return errorReply("failed to set IPv4 subnet: %v", err)
					} else if newNetwork, err := vz.NewVmnetNetwork(config); err != nil {
						return errorReply("failed to create VmnetNetwork: %v", err)
					} else if rawSerialization, err := newNetwork.CopySerialization(); err != nil {
						return errorReply("failed to copy network serialization: %v", err)
					} else if serialization = xpc.NewObject(rawSerialization); serialization == nil {
						return errorReply("failed to create XPC object from serialization")
					}
					serializationStore[prefix] = serialization
				}
				return msg.DictionaryCreateReply(
					xpc.WithValue("Network", serialization),
				)
			},
			nil,
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
	<-ctx.Done()
	return nil
}

// RequestSharedVmnetNetwork requests a shared VmnetNetwork for the given subnet
// from the mach service "io.lima-vm.vz.vmnet.shared.subnet".
func RequestSharedVmnetNetwork(ctx context.Context, subnet netip.Prefix) (*vz.VmnetNetwork, error) {
	session, err := xpc.NewSession(MachServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create xpc session to %q: %w", MachServiceName, err)
	}
	defer session.Cancel()
	reply, err := session.SendDictionaryWithReply(
		ctx,
		xpc.WithString("Subnet", subnet.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send xpc message to %q: %w", MachServiceName, err)
	}
	if errMsg := reply.DictionaryGetString("Error"); errMsg != "" {
		return nil, fmt.Errorf("error from mach service %q: %s", MachServiceName, errMsg)
	}
	serialization := reply.DictionaryGetValue("Network")
	if serialization == nil {
		return nil, fmt.Errorf("no Network object in reply from %q", MachServiceName)
	}
	network, err := vz.NewVmnetNetworkWithSerialization(serialization.XpcObject)
	if err != nil {
		return nil, fmt.Errorf("failed to create VmnetNetwork (%v) from serialization: %w", subnet, err)
	}
	return network, nil
}
