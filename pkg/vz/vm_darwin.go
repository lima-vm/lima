//go:build darwin && !no_vz
// +build darwin,!no_vz

package vz

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

func startVM(ctx context.Context, driver *driver.BaseDriver) (*vz.VirtualMachine, chan error, error) {
	server, client, err := createSockPair()
	if err != nil {
		return nil, nil, err
	}
	machine, err := createVM(driver, client)
	if err != nil {
		return nil, nil, err
	}

	fileConn, err := net.FileConn(server)
	if err != nil {
		return nil, nil, err
	}

	err = machine.Start()

	networks.StartGVisorNetstack(ctx, &networks.GVisorNetstackOpts{
		Conn:         fileConn,
		MTU:          1500,
		SSHLocalPort: driver.SSHLocalPort,
		MacAddress:   limayaml.MACAddress(driver.Instance.Dir),
		Stream:       false,
	})
	if err != nil {
		return nil, nil, err
	}

	errCh := make(chan error)
	go func() {
		//Handle errors via errCh and handle stop vm during context close

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
					pidFile := filepath.Join(driver.Instance.Dir, filenames.PIDFile(*driver.Yaml.VMType))
					if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
						logrus.Errorf("pidfile %q already exists", pidFile)
						errCh <- err
					}
					if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
						logrus.Errorf("error writing to pid fil %q", pidFile)
						errCh <- err
					}
					defer os.RemoveAll(pidFile)
					logrus.Info("[VZ] - vm state change: running")
				case vz.VirtualMachineStateStopped:
					logrus.Info("[VZ] - vm state change: stopped")
					errCh <- errors.New("vz driver state stopped")
				default:
					logrus.Debugf("[VZ] - vm state change: %q", newState)
				}
			}
		}
	}()

	return machine, errCh, err
}

func createVM(driver *driver.BaseDriver, networkConn *os.File) (*vz.VirtualMachine, error) {
	vmConfig, err := createInitialConfig(driver)
	if err != nil {
		return nil, err
	}

	if err = attachPlatformConfig(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachSerialPort(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachNetwork(driver, vmConfig, networkConn); err != nil {
		return nil, err
	}

	if err = attachDisks(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachDisplay(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachFolderMounts(driver, vmConfig); err != nil {
		return nil, err
	}

	if err = attachOtherDevices(driver, vmConfig); err != nil {
		return nil, err
	}

	validated, err := vmConfig.Validate()
	if !validated || err != nil {
		return nil, err
	}

	return vz.NewVirtualMachine(vmConfig)
}

func createInitialConfig(driver *driver.BaseDriver) (*vz.VirtualMachineConfiguration, error) {
	efiVariableStore, err := getEFI(driver)
	if err != nil {
		return nil, err
	}

	bootLoader, err := vz.NewEFIBootLoader(vz.WithEFIVariableStore(efiVariableStore))
	if err != nil {
		return nil, err
	}

	bytes, err := units.RAMInBytes(*driver.Yaml.Memory)
	if err != nil {
		return nil, err
	}

	vmConfig, err := vz.NewVirtualMachineConfiguration(
		bootLoader,
		uint(*driver.Yaml.CPUs),
		uint64(bytes),
	)
	if err != nil {
		return nil, err
	}
	return vmConfig, nil
}

func attachPlatformConfig(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	machineIdentifier, err := getMachineIdentifier(driver)
	if err != nil {
		return err
	}

	platformConfig, err := vz.NewGenericPlatformConfiguration(vz.WithGenericMachineIdentifier(machineIdentifier))
	if err != nil {
		return err
	}
	vmConfig.SetPlatformVirtualMachineConfiguration(platformConfig)
	return nil
}

func attachSerialPort(driver *driver.BaseDriver, config *vz.VirtualMachineConfiguration) error {
	path := filepath.Join(driver.Instance.Dir, filenames.SerialLog)
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

func attachNetwork(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration, networkConn *os.File) error {
	//slirp network using gvisor netstack
	fileAttachment, err := vz.NewFileHandleNetworkDeviceAttachment(networkConn)
	if err != nil {
		return err
	}
	err = fileAttachment.SetMaximumTransmissionUnit(1500)
	if err != nil {
		return err
	}
	networkConfig, err := newVirtioNetworkDeviceConfiguration(fileAttachment, limayaml.MACAddress(driver.Instance.Dir))
	if err != nil {
		return err
	}
	configurations := []*vz.VirtioNetworkDeviceConfiguration{
		networkConfig,
	}
	for _, nw := range driver.Instance.Networks {
		if nw.VZNAT != nil && *nw.VZNAT {
			attachment, err := vz.NewNATNetworkDeviceAttachment()
			if err != nil {
				return err
			}
			networkConfig, err = newVirtioNetworkDeviceConfiguration(attachment, nw.MACAddress)
			if err != nil {
				return err
			}
			configurations = append(configurations, networkConfig)
		}

		if nw.Lima != "" {
			nwCfg, err := networks.Config()
			if err != nil {
				return err
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
				attachment, err := vz.NewFileHandleNetworkDeviceAttachment(clientFile)
				if err != nil {
					return err
				}
				networkConfig, err = newVirtioNetworkDeviceConfiguration(attachment, nw.MACAddress)
				if err != nil {
					return err
				}
				configurations = append(configurations, networkConfig)
			}
		} else if nw.Socket != "" {
			clientFile, err := DialQemu(nw.Socket)
			if err != nil {
				return err
			}
			attachment, err := vz.NewFileHandleNetworkDeviceAttachment(clientFile)
			if err != nil {
				return err
			}
			networkConfig, err = newVirtioNetworkDeviceConfiguration(attachment, nw.MACAddress)
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
	format, err := imgutil.DetectFormat(diskPath)
	if err != nil {
		return fmt.Errorf("failed to detect the format of %q: %w", diskPath, err)
	}
	if format != "raw" {
		return fmt.Errorf("expected the format of %q to be \"raw\", got %q", diskPath, format)
	}
	// TODO: ensure that the disk is formatted with GPT or ISO9660
	return nil
}

func attachDisks(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	baseDiskPath := filepath.Join(driver.Instance.Dir, filenames.BaseDisk)
	diffDiskPath := filepath.Join(driver.Instance.Dir, filenames.DiffDisk)
	ciDataPath := filepath.Join(driver.Instance.Dir, filenames.CIDataISO)
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
	diffDiskAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(diffDiskPath, false, vz.DiskImageCachingModeAutomatic, vz.DiskImageSynchronizationModeFsync)
	if err != nil {
		return err
	}
	diffDisk, err := vz.NewVirtioBlockDeviceConfiguration(diffDiskAttachment)
	if err != nil {
		return err
	}
	configurations = append(configurations, diffDisk)

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

func attachDisplay(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	if driver.Yaml.Video.Display != nil {
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
	}
	return nil
}

func attachFolderMounts(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	var mounts []vz.DirectorySharingDeviceConfiguration
	if *driver.Yaml.MountType == limayaml.VIRTIOFS {
		for i, mount := range driver.Yaml.Mounts {
			expandedPath, err := localpathutil.Expand(mount.Location)
			if err != nil {
				return err
			}
			if _, err := os.Stat(expandedPath); errors.Is(err, os.ErrNotExist) {
				err := os.MkdirAll(expandedPath, 0750)
				if err != nil {
					return err
				}
			}

			directory, err := vz.NewSharedDirectory(expandedPath, !*mount.Writable)
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

	if *driver.Yaml.Rosetta.Enabled {
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

func attachOtherDevices(_ *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
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

func getMachineIdentifier(driver *driver.BaseDriver) (*vz.GenericMachineIdentifier, error) {
	identifier := filepath.Join(driver.Instance.Dir, filenames.VzIdentifier)
	if _, err := os.Stat(identifier); os.IsNotExist(err) {
		machineIdentifier, err := vz.NewGenericMachineIdentifier()
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(identifier, machineIdentifier.DataRepresentation(), 0666)
		if err != nil {
			return nil, err
		}
		return machineIdentifier, nil
	}
	return vz.NewGenericMachineIdentifierWithDataPath(identifier)
}

func getEFI(driver *driver.BaseDriver) (*vz.EFIVariableStore, error) {
	efi := filepath.Join(driver.Instance.Dir, filenames.VzEfi)
	if _, err := os.Stat(efi); os.IsNotExist(err) {
		return vz.NewEFIVariableStore(efi, vz.WithCreatingEFIVariableStore())
	}
	return vz.NewEFIVariableStore(efi)
}

func createSockPair() (*os.File, *os.File, error) {
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
	return os.NewFile(uintptr(serverFD), "server"), os.NewFile(uintptr(clientFD), "client"), nil
}
