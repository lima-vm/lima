//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"github.com/coreos/go-semver/semver"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

// diskImageCachingMode is set to DiskImageCachingModeCached so as to avoid disk corruption on ARM:
// - https://github.com/utmapp/UTM/issues/4840#issuecomment-1824340975
// - https://github.com/utmapp/UTM/issues/4840#issuecomment-1824542732
//
// Eventually we may bring this back to DiskImageCachingModeAutomatic when the corruption issue is properly fixed.
const diskImageCachingMode = vz.DiskImageCachingModeCached

type virtualMachineWrapper struct {
	*vz.VirtualMachine
	mu      sync.Mutex
	stopped bool
}

// Hold all *os.File created via socketpair() so that they won't get garbage collected. f.FD() gets invalid if f gets garbage collected.
var vmNetworkFiles = make([]*os.File, 1)

func startVM(ctx context.Context, inst *store.Instance, sshLocalPort int) (*virtualMachineWrapper, chan error, error) {
	usernetClient, err := startUsernet(ctx, inst)
	if err != nil {
		return nil, nil, err
	}

	machine, err := createVM(inst)
	if err != nil {
		return nil, nil, err
	}

	err = machine.Start()
	if err != nil {
		return nil, nil, err
	}

	wrapper := &virtualMachineWrapper{VirtualMachine: machine, stopped: false}

	errCh := make(chan error)

	filesToRemove := make(map[string]struct{})
	defer func() {
		for f := range filesToRemove {
			_ = os.RemoveAll(f)
		}
	}()
	go func() {
		// Handle errors via errCh and handle stop vm during context close
		defer func() {
			for i := range vmNetworkFiles {
				vmNetworkFiles[i].Close()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				logrus.Info("Context closed, stopping vm")
				if machine.CanStop() {
					_, err := machine.RequestStop()
					logrus.Errorf("Error while stopping the VM %q", err)
				}
			case newState := <-machine.StateChangedNotify():
				switch newState {
				case vz.VirtualMachineStateRunning:
					pidFile := filepath.Join(inst.Dir, filenames.PIDFile(*inst.Config.VMType))
					if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
						logrus.Errorf("pidfile %q already exists", pidFile)
						errCh <- err
					}
					if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
						logrus.Errorf("error writing to pid fil %q", pidFile)
						errCh <- err
					}
					filesToRemove[pidFile] = struct{}{}
					logrus.Info("[VZ] - vm state change: running")

					err := usernetClient.ConfigureDriver(ctx, inst, sshLocalPort)
					if err != nil {
						errCh <- err
					}
				case vz.VirtualMachineStateStopped:
					logrus.Info("[VZ] - vm state change: stopped")
					wrapper.mu.Lock()
					wrapper.stopped = true
					wrapper.mu.Unlock()
					_ = usernetClient.UnExposeSSH(inst.SSHLocalPort)
					errCh <- errors.New("vz driver state stopped")
				default:
					logrus.Debugf("[VZ] - vm state change: %q", newState)
				}
			}
		}
	}()

	return wrapper, errCh, err
}

func startUsernet(ctx context.Context, inst *store.Instance) (*usernet.Client, error) {
	if firstUsernetIndex := limayaml.FirstUsernetIndex(inst.Config); firstUsernetIndex != -1 {
		nwName := inst.Config.Networks[firstUsernetIndex].Lima
		return usernet.NewClientByName(nwName), nil
	}
	// Start a in-process gvisor-tap-vsock
	endpointSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.EndpointSock)
	if err != nil {
		return nil, err
	}
	vzSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.FDSock)
	if err != nil {
		return nil, err
	}
	os.RemoveAll(endpointSock)
	os.RemoveAll(vzSock)
	err = usernet.StartGVisorNetstack(ctx, &usernet.GVisorNetstackOpts{
		MTU:      1500,
		Endpoint: endpointSock,
		FdSocket: vzSock,
		Async:    true,
		DefaultLeases: map[string]string{
			networks.SlirpIPAddress: limayaml.MACAddress(inst.Dir),
		},
		Subnet: networks.SlirpNetwork,
	})
	if err != nil {
		return nil, err
	}
	subnetIP, _, err := net.ParseCIDR(networks.SlirpNetwork)
	return usernet.NewClient(endpointSock, subnetIP), err
}

func createVM(inst *store.Instance) (*vz.VirtualMachine, error) {
	vmConfig, err := createInitialConfig(inst)
	if err != nil {
		return nil, err
	}

	if err = attachPlatformConfig(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachSerialPort(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachNetwork(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachDisks(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachDisplay(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachFolderMounts(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachAudio(inst, vmConfig); err != nil {
		return nil, err
	}

	if err = attachOtherDevices(inst, vmConfig); err != nil {
		return nil, err
	}

	validated, err := vmConfig.Validate()
	if !validated || err != nil {
		return nil, err
	}

	return vz.NewVirtualMachine(vmConfig)
}

func createInitialConfig(inst *store.Instance) (*vz.VirtualMachineConfiguration, error) {
	bootLoader, err := bootLoader(inst)
	if err != nil {
		return nil, err
	}

	bytes, err := units.RAMInBytes(*inst.Config.Memory)
	if err != nil {
		return nil, err
	}

	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(*inst.Config.CPUs),
		uint64(bytes),
	)
	if err != nil {
		return nil, err
	}
	return vmConfig, nil
}

func attachPlatformConfig(inst *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	machineIdentifier, err := getMachineIdentifier(inst)
	if err != nil {
		return err
	}

	platformConfig, err := vz.NewGenericPlatformConfiguration(vz.WithGenericMachineIdentifier(machineIdentifier))
	if err != nil {
		return err
	}

	// nested virt
	if *inst.Config.NestedVirtualization {
		macOSProductVersion, err := osutil.ProductVersion()
		if err != nil {
			return fmt.Errorf("failed to get macOS product version: %w", err)
		}

		if macOSProductVersion.LessThan(*semver.New("15.0.0")) {
			return errors.New("nested virtualization requires macOS 15 or newer")
		}

		if !vz.IsNestedVirtualizationSupported() {
			return errors.New("nested virtualization is not supported on this device")
		}

		if err := platformConfig.SetNestedVirtualizationEnabled(true); err != nil {
			return fmt.Errorf("cannot enable nested virtualization: %w", err)
		}
	}

	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)
	return nil
}

func attachSerialPort(inst *store.Instance, config *vz.VirtualMachineConfiguration) error {
	path := filepath.Join(inst.Dir, filenames.SerialVirtioLog)
	serialPortAttachment, err := vz.NewFileSerialPortAttachment(path, false)
	if err != nil {
		return err
	}
	consoleConfig, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(serialPortAttachment)
	config.SetSerialPortsVirtualMachineConfiguration([]*vz.VirtioConsoleDeviceSerialPortConfiguration{
		consoleConfig,
	})
	return err
}

func newVirtioFileNetworkDeviceConfiguration(file *os.File, macStr string) (*vz.VirtioNetworkDeviceConfiguration, error) {
	fileAttachment, err := vz.NewFileHandleNetworkDeviceAttachment(file)
	if err != nil {
		return nil, err
	}
	return newVirtioNetworkDeviceConfiguration(fileAttachment, macStr)
}

func newVirtioNetworkDeviceConfiguration(attachment vz.NetworkDeviceAttachment, macStr string) (*vz.VirtioNetworkDeviceConfiguration, error) {
	networkConfig, err := vz.NewVirtioNetworkDeviceConfiguration(attachment)
	if err != nil {
		return nil, err
	}
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return nil, err
	}
	address, err := vz.NewMACAddress(mac)
	if err != nil {
		return nil, err
	}
	networkConfig.SetMACAddress(address)
	return networkConfig, nil
}

func attachNetwork(inst *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	var configurations []*vz.VirtioNetworkDeviceConfiguration

	// Configure default usernetwork with limayaml.MACAddress(inst.Dir) for eth0 interface
	firstUsernetIndex := limayaml.FirstUsernetIndex(inst.Config)
	if firstUsernetIndex == -1 {
		// slirp network using gvisor netstack
		vzSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.FDSock)
		if err != nil {
			return err
		}
		networkConn, err := PassFDToUnix(vzSock)
		if err != nil {
			return err
		}
		networkConfig, err := newVirtioFileNetworkDeviceConfiguration(networkConn, limayaml.MACAddress(inst.Dir))
		if err != nil {
			return err
		}
		configurations = append(configurations, networkConfig)
	} else {
		vzSock, err := usernet.Sock(inst.Config.Networks[firstUsernetIndex].Lima, usernet.FDSock)
		if err != nil {
			return err
		}
		networkConn, err := PassFDToUnix(vzSock)
		if err != nil {
			return err
		}
		networkConfig, err := newVirtioFileNetworkDeviceConfiguration(networkConn, limayaml.MACAddress(inst.Dir))
		if err != nil {
			return err
		}
		configurations = append(configurations, networkConfig)
	}

	for i, nw := range inst.Networks {
		if nw.VZNAT != nil && *nw.VZNAT {
			attachment, err := vz.NewNATNetworkDeviceAttachment()
			if err != nil {
				return err
			}
			networkConfig, err := newVirtioNetworkDeviceConfiguration(attachment, nw.MACAddress)
			if err != nil {
				return err
			}
			configurations = append(configurations, networkConfig)
		} else if nw.Lima != "" {
			nwCfg, err := networks.LoadConfig()
			if err != nil {
				return err
			}
			isUsernet, err := nwCfg.Usernet(nw.Lima)
			if err != nil {
				return err
			}
			if isUsernet {
				if i == firstUsernetIndex {
					continue
				}
				vzSock, err := usernet.Sock(nw.Lima, usernet.FDSock)
				if err != nil {
					return err
				}
				clientFile, err := PassFDToUnix(vzSock)
				if err != nil {
					return err
				}
				networkConfig, err := newVirtioFileNetworkDeviceConfiguration(clientFile, nw.MACAddress)
				if err != nil {
					return err
				}
				configurations = append(configurations, networkConfig)
			} else {
				if runtime.GOOS != "darwin" {
					return fmt.Errorf("networks.yaml '%s' configuration is only supported on macOS right now", nw.Lima)
				}
				socketVMNetOk, err := nwCfg.IsDaemonInstalled(networks.SocketVMNet)
				if err != nil {
					return err
				}
				if socketVMNetOk {
					logrus.Debugf("Using socketVMNet (%q)", nwCfg.Paths.SocketVMNet)
					sock, err := networks.Sock(nw.Lima)
					if err != nil {
						return err
					}

					clientFile, err := DialQemu(sock)
					if err != nil {
						return err
					}
					networkConfig, err := newVirtioFileNetworkDeviceConfiguration(clientFile, nw.MACAddress)
					if err != nil {
						return err
					}
					configurations = append(configurations, networkConfig)
				}
			}
		} else if nw.Socket != "" {
			clientFile, err := DialQemu(nw.Socket)
			if err != nil {
				return err
			}
			networkConfig, err := newVirtioFileNetworkDeviceConfiguration(clientFile, nw.MACAddress)
			if err != nil {
				return err
			}
			configurations = append(configurations, networkConfig)
		}
	}
	vmConfig.SetNetworkDevicesVirtualMachineConfiguration(configurations)
	return nil
}

func validateDiskFormat(diskPath string) error {
	f, err := os.Open(diskPath)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := qcow2reader.Open(f)
	if err != nil {
		return fmt.Errorf("failed to detect the format of %q: %w", diskPath, err)
	}
	if t := img.Type(); t != raw.Type {
		return fmt.Errorf("expected the format of %q to be %q, got %q", diskPath, raw.Type, t)
	}
	// TODO: ensure that the disk is formatted with GPT or ISO9660
	return nil
}

func attachDisks(inst *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	baseDiskPath := filepath.Join(inst.Dir, filenames.BaseDisk)
	diffDiskPath := filepath.Join(inst.Dir, filenames.DiffDisk)
	ciDataPath := filepath.Join(inst.Dir, filenames.CIDataISO)
	isBaseDiskCDROM, err := iso9660util.IsISO9660(baseDiskPath)
	if err != nil {
		return err
	}
	var configurations []vz.StorageDeviceConfiguration

	if isBaseDiskCDROM {
		if err = validateDiskFormat(baseDiskPath); err != nil {
			return err
		}
		baseDiskAttachment, err := vz.NewDiskImageStorageDeviceAttachment(baseDiskPath, true)
		if err != nil {
			return err
		}
		baseDisk, err := vz.NewUSBMassStorageDeviceConfiguration(baseDiskAttachment)
		if err != nil {
			return err
		}
		configurations = append(configurations, baseDisk)
	}
	if err = validateDiskFormat(diffDiskPath); err != nil {
		return err
	}
	diffDiskAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(diffDiskPath, false, diskImageCachingMode, vz.DiskImageSynchronizationModeFsync)
	if err != nil {
		return err
	}
	diffDisk, err := vz.NewVirtioBlockDeviceConfiguration(diffDiskAttachment)
	if err != nil {
		return err
	}
	configurations = append(configurations, diffDisk)

	diskUtil := proxyimgutil.NewDiskUtil()

	for _, d := range inst.Config.AdditionalDisks {
		diskName := d.Name
		disk, err := store.InspectDisk(diskName)
		if err != nil {
			return fmt.Errorf("failed to run load disk %q: %w", diskName, err)
		}

		if disk.Instance != "" {
			return fmt.Errorf("failed to run attach disk %q, in use by instance %q", diskName, disk.Instance)
		}
		logrus.Infof("Mounting disk %q on %q", diskName, disk.MountPoint)
		err = disk.Lock(inst.Dir)
		if err != nil {
			return fmt.Errorf("failed to run lock disk %q: %w", diskName, err)
		}
		extraDiskPath := filepath.Join(disk.Dir, filenames.DataDisk)
		// ConvertToRaw is a NOP if no conversion is needed
		logrus.Debugf("Converting extra disk %q to a raw disk (if it is not a raw)", extraDiskPath)

		if err = diskUtil.ConvertToRaw(extraDiskPath, extraDiskPath, nil, true); err != nil {
			return fmt.Errorf("failed to convert extra disk %q to a raw disk: %w", extraDiskPath, err)
		}
		extraDiskPathAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(extraDiskPath, false, diskImageCachingMode, vz.DiskImageSynchronizationModeFsync)
		if err != nil {
			return fmt.Errorf("failed to create disk attachment for extra disk %q: %w", extraDiskPath, err)
		}
		extraDisk, err := vz.NewVirtioBlockDeviceConfiguration(extraDiskPathAttachment)
		if err != nil {
			return fmt.Errorf("failed to create new virtio block device config for extra disk %q: %w", extraDiskPath, err)
		}
		configurations = append(configurations, extraDisk)
	}

	if err = validateDiskFormat(ciDataPath); err != nil {
		return err
	}
	ciDataAttachment, err := vz.NewDiskImageStorageDeviceAttachment(ciDataPath, true)
	if err != nil {
		return err
	}
	ciData, err := vz.NewVirtioBlockDeviceConfiguration(ciDataAttachment)
	if err != nil {
		return err
	}
	configurations = append(configurations, ciData)

	vmConfig.SetStorageDevicesVirtualMachineConfiguration(configurations)
	return nil
}

func attachDisplay(inst *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	switch *inst.Config.Video.Display {
	case "vz", "default":
		graphicsDeviceConfiguration, err := vz.NewVirtioGraphicsDeviceConfiguration()
		if err != nil {
			return err
		}
		scanoutConfiguration, err := vz.NewVirtioGraphicsScanoutConfiguration(1920, 1200)
		if err != nil {
			return err
		}
		graphicsDeviceConfiguration.SetScanouts(scanoutConfiguration)

		vmConfig.SetGraphicsDevicesVirtualMachineConfiguration([]vz.GraphicsDeviceConfiguration{
			graphicsDeviceConfiguration,
		})
		return nil
	case "none":
		return nil
	default:
		return fmt.Errorf("unexpected video display %q", *inst.Config.Video.Display)
	}
}

func attachFolderMounts(inst *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	var mounts []vz.DirectorySharingDeviceConfiguration
	if *inst.Config.MountType == limayaml.VIRTIOFS {
		for i, mount := range inst.Config.Mounts {
			if _, err := os.Stat(mount.Location); errors.Is(err, os.ErrNotExist) {
				err := os.MkdirAll(mount.Location, 0o750)
				if err != nil {
					return err
				}
			}

			directory, err := vz.NewSharedDirectory(mount.Location, !*mount.Writable)
			if err != nil {
				return err
			}
			share, err := vz.NewSingleDirectoryShare(directory)
			if err != nil {
				return err
			}

			tag := fmt.Sprintf("mount%d", i)
			config, err := vz.NewVirtioFileSystemDeviceConfiguration(tag)
			if err != nil {
				return err
			}
			config.SetDirectoryShare(share)
			mounts = append(mounts, config)
		}
	}

	if *inst.Config.Rosetta.Enabled {
		logrus.Info("Setting up Rosetta share")
		directorySharingDeviceConfig, err := createRosettaDirectoryShareConfiguration()
		if err != nil {
			logrus.Warnf("Unable to configure Rosetta: %s", err)
		} else {
			mounts = append(mounts, directorySharingDeviceConfig)
		}
	}

	if len(mounts) > 0 {
		vmConfig.SetDirectorySharingDevicesVirtualMachineConfiguration(mounts)
	}
	return nil
}

func attachAudio(inst *store.Instance, config *vz.VirtualMachineConfiguration) error {
	switch *inst.Config.Audio.Device {
	case "vz", "default":
		outputStream, err := vz.NewVirtioSoundDeviceHostOutputStreamConfiguration()
		if err != nil {
			return err
		}
		soundDeviceConfiguration, err := vz.NewVirtioSoundDeviceConfiguration()
		if err != nil {
			return err
		}
		soundDeviceConfiguration.SetStreams(outputStream)
		config.SetAudioDevicesVirtualMachineConfiguration([]vz.AudioDeviceConfiguration{
			soundDeviceConfiguration,
		})
		return nil
	case "", "none":
		return nil
	default:
		return fmt.Errorf("unexpected audio device %q", *inst.Config.Audio.Device)
	}
}

func attachOtherDevices(_ *store.Instance, vmConfig *vz.VirtualMachineConfiguration) error {
	entropyConfig, err := vz.NewVirtioEntropyDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetEntropyDevicesVirtualMachineConfiguration([]*vz.VirtioEntropyDeviceConfiguration{
		entropyConfig,
	})

	configuration, err := vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetMemoryBalloonDevicesVirtualMachineConfiguration([]vz.MemoryBalloonDeviceConfiguration{
		configuration,
	})

	deviceConfiguration, err := vz.NewVirtioSocketDeviceConfiguration()
	vmConfig.SetSocketDevicesVirtualMachineConfiguration([]vz.SocketDeviceConfiguration{
		deviceConfiguration,
	})
	if err != nil {
		return err
	}

	// Set audio device
	inputAudioDeviceConfig, err := vz.NewVirtioSoundDeviceConfiguration()
	if err != nil {
		return err
	}
	inputStream, err := vz.NewVirtioSoundDeviceHostInputStreamConfiguration()
	if err != nil {
		return err
	}
	inputAudioDeviceConfig.SetStreams(
		inputStream,
	)

	outputAudioDeviceConfig, err := vz.NewVirtioSoundDeviceConfiguration()
	if err != nil {
		return err
	}
	outputStream, err := vz.NewVirtioSoundDeviceHostOutputStreamConfiguration()
	if err != nil {
		return err
	}
	outputAudioDeviceConfig.SetStreams(
		outputStream,
	)
	vmConfig.SetAudioDevicesVirtualMachineConfiguration([]vz.AudioDeviceConfiguration{
		inputAudioDeviceConfig,
		outputAudioDeviceConfig,
	})

	// Set pointing device
	pointingDeviceConfig, err := vz.NewUSBScreenCoordinatePointingDeviceConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetPointingDevicesVirtualMachineConfiguration([]vz.PointingDeviceConfiguration{
		pointingDeviceConfig,
	})

	// Set keyboard device
	keyboardDeviceConfig, err := vz.NewUSBKeyboardConfiguration()
	if err != nil {
		return err
	}
	vmConfig.SetKeyboardsVirtualMachineConfiguration([]vz.KeyboardConfiguration{
		keyboardDeviceConfig,
	})
	return nil
}

func getMachineIdentifier(inst *store.Instance) (*vz.GenericMachineIdentifier, error) {
	identifier := filepath.Join(inst.Dir, filenames.VzIdentifier)
	if _, err := os.Stat(identifier); os.IsNotExist(err) {
		machineIdentifier, err := vz.NewGenericMachineIdentifier()
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(identifier, machineIdentifier.DataRepresentation(), 0o666)
		if err != nil {
			return nil, err
		}
		return machineIdentifier, nil
	}
	return vz.NewGenericMachineIdentifierWithDataPath(identifier)
}

func bootLoader(inst *store.Instance) (vz.BootLoader, error) {
	linuxBootLoader, err := linuxBootLoader(inst)
	if linuxBootLoader != nil {
		return linuxBootLoader, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	efiVariableStore, err := getEFI(inst)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Using EFI Boot Loader")
	return vz.NewEFIBootLoader(vz.WithEFIVariableStore(efiVariableStore))
}

func linuxBootLoader(inst *store.Instance) (*vz.LinuxBootLoader, error) {
	kernel := filepath.Join(inst.Dir, filenames.Kernel)
	kernelCmdline := filepath.Join(inst.Dir, filenames.KernelCmdline)
	initrd := filepath.Join(inst.Dir, filenames.Initrd)
	if _, err := os.Stat(kernel); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Debugf("Kernel file %q not found", kernel)
		} else {
			logrus.WithError(err).Debugf("Error while checking kernel file %q", kernel)
		}
		return nil, err
	}
	var opt []vz.LinuxBootLoaderOption
	if b, err := os.ReadFile(kernelCmdline); err == nil {
		logrus.Debugf("Using kernel command line %q", string(b))
		opt = append(opt, vz.WithCommandLine(string(b)))
	}
	if _, err := os.Stat(initrd); err == nil {
		logrus.Debugf("Using initrd %q", initrd)
		opt = append(opt, vz.WithInitrd(initrd))
	}
	logrus.Debugf("Using Linux Boot Loader with kernel %q", kernel)
	return vz.NewLinuxBootLoader(kernel, opt...)
}

func getEFI(inst *store.Instance) (*vz.EFIVariableStore, error) {
	efi := filepath.Join(inst.Dir, filenames.VzEfi)
	if _, err := os.Stat(efi); os.IsNotExist(err) {
		return vz.NewEFIVariableStore(efi, vz.WithCreatingEFIVariableStore())
	}
	return vz.NewEFIVariableStore(efi)
}

func createSockPair() (server, client *os.File, _ error) {
	pairs, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return nil, nil, err
	}
	serverFD := pairs[0]
	clientFD := pairs[1]

	if err = syscall.SetsockoptInt(serverFD, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(serverFD, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(clientFD, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 1*1024*1024); err != nil {
		return nil, nil, err
	}
	if err = syscall.SetsockoptInt(clientFD, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 4*1024*1024); err != nil {
		return nil, nil, err
	}
	server = os.NewFile(uintptr(serverFD), "server")
	client = os.NewFile(uintptr(clientFD), "client")
	runtime.SetFinalizer(server, func(*os.File) {
		logrus.Debugf("Server network file GC'ed")
	})
	runtime.SetFinalizer(client, func(*os.File) {
		logrus.Debugf("Client network file GC'ed")
	})
	vmNetworkFiles = append(vmNetworkFiles, server, client)
	return server, client, nil
}
