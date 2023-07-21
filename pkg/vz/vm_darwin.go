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
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/Code-Hex/vz/v3"
	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/nativeimgutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

type virtualMachineWrapper struct {
	*vz.VirtualMachine
	mu      sync.Mutex
	stopped bool
}

// Hold all *os.File created via socketpair() so that they won't get garbage collected. f.FD() gets invalid if f gets garbage collected.
var vmNetworkFiles = make([]*os.File, 1)

func startVM(ctx context.Context, driver *driver.BaseDriver) (*virtualMachineWrapper, chan error, error) {
	usernetClient, err := startUsernet(ctx, driver)
	if err != nil {
		return nil, nil, err
	}

	machine, err := createVM(driver)
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
		//Handle errors via errCh and handle stop vm during context close
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
					pidFile := filepath.Join(driver.Instance.Dir, filenames.PIDFile(*driver.Yaml.VMType))
					if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
						logrus.Errorf("pidfile %q already exists", pidFile)
						errCh <- err
					}
					if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
						logrus.Errorf("error writing to pid fil %q", pidFile)
						errCh <- err
					}
					filesToRemove[pidFile] = struct{}{}
					logrus.Info("[VZ] - vm state change: running")

					err := usernetClient.ResolveAndForwardSSH(limayaml.MACAddress(driver.Instance.Dir), driver.SSHLocalPort)
					if err != nil {
						errCh <- err
					}
				case vz.VirtualMachineStateStopped:
					logrus.Info("[VZ] - vm state change: stopped")
					wrapper.mu.Lock()
					wrapper.stopped = true
					wrapper.mu.Unlock()
					usernetClient.UnExposeSSH(driver.SSHLocalPort)
					errCh <- errors.New("vz driver state stopped")
				default:
					logrus.Debugf("[VZ] - vm state change: %q", newState)
				}
			}
		}
	}()

	return wrapper, errCh, err
}

func startUsernet(ctx context.Context, driver *driver.BaseDriver) (*usernet.Client, error) {
	firstUsernetIndex := limayaml.FirstUsernetIndex(driver.Yaml)
	if firstUsernetIndex == -1 {
		//Start a in-process gvisor-tap-vsock
		endpointSock := usernet.SockWithDirectory(driver.Instance.Dir, "", usernet.EndpointSock)
		vzSock := usernet.SockWithDirectory(driver.Instance.Dir, "", usernet.FDSock)
		os.RemoveAll(endpointSock)
		os.RemoveAll(vzSock)
		err := usernet.StartGVisorNetstack(ctx, &usernet.GVisorNetstackOpts{
			MTU:      1500,
			Endpoint: endpointSock,
			FdSocket: vzSock,
			Async:    true,
			DefaultLeases: map[string]string{
				networks.SlirpIPAddress: limayaml.MACAddress(driver.Instance.Dir),
			},
		})
		if err != nil {
			return nil, err
		}
		return usernet.NewClient(endpointSock), nil
	}
	endpointSock, err := usernet.Sock(driver.Yaml.Networks[firstUsernetIndex].Lima, usernet.EndpointSock)
	if err != nil {
		return nil, err
	}
	return usernet.NewClient(endpointSock), nil
}

func createVM(driver *driver.BaseDriver) (*vz.VirtualMachine, error) {
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

	if err = attachNetwork(driver, vmConfig); err != nil {
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

	if err = attachAudio(driver, vmConfig); err != nil {
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
	path := filepath.Join(driver.Instance.Dir, filenames.SerialVirtioLog)
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

func attachNetwork(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	var configurations []*vz.VirtioNetworkDeviceConfiguration

	//Configure default usernetwork with limayaml.MACAddress(driver.Instance.Dir) for eth0 interface
	firstUsernetIndex := limayaml.FirstUsernetIndex(driver.Yaml)
	if firstUsernetIndex == -1 {
		//slirp network using gvisor netstack
		vzSock := usernet.SockWithDirectory(driver.Instance.Dir, "", usernet.FDSock)
		networkConn, err := PassFDToUnix(vzSock)
		if err != nil {
			return err
		}
		networkConfig, err := newVirtioFileNetworkDeviceConfiguration(networkConn, limayaml.MACAddress(driver.Instance.Dir))
		if err != nil {
			return err
		}
		configurations = append(configurations, networkConfig)
	} else {
		vzSock, err := usernet.Sock(driver.Yaml.Networks[firstUsernetIndex].Lima, usernet.FDSock)
		if err != nil {
			return err
		}
		networkConn, err := PassFDToUnix(vzSock)
		if err != nil {
			return err
		}
		networkConfig, err := newVirtioFileNetworkDeviceConfiguration(networkConn, limayaml.MACAddress(driver.Instance.Dir))
		if err != nil {
			return err
		}
		configurations = append(configurations, networkConfig)
	}

	for i, nw := range driver.Instance.Networks {
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
			nwCfg, err := networks.Config()
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

	for _, diskName := range driver.Yaml.AdditionalDisks {
		d, err := store.InspectDisk(diskName)
		if err != nil {
			return fmt.Errorf("failed to run load disk %q: %q", diskName, err)
		}

		if d.Instance != "" {
			return fmt.Errorf("failed to run attach disk %q, in use by instance %q", diskName, d.Instance)
		}
		logrus.Infof("Mounting disk %q on %q", diskName, d.MountPoint)
		err = d.Lock(driver.Instance.Dir)
		if err != nil {
			return fmt.Errorf("failed to run lock disk %q: %q", diskName, err)
		}
		extraDiskPath := filepath.Join(d.Dir, filenames.DataDisk)
		// ConvertToRaw is a NOP if no conversion is needed
		logrus.Debugf("Converting extra disk %q to a raw disk (if it is not a raw)", extraDiskPath)
		if err = nativeimgutil.ConvertToRaw(extraDiskPath, extraDiskPath, nil, true); err != nil {
			return fmt.Errorf("failed to convert extra disk %q to a raw disk: %w", extraDiskPath, err)
		}
		extraDiskPathAttachment, err := vz.NewDiskImageStorageDeviceAttachmentWithCacheAndSync(extraDiskPath, false, vz.DiskImageCachingModeAutomatic, vz.DiskImageSynchronizationModeFsync)
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

func attachDisplay(driver *driver.BaseDriver, vmConfig *vz.VirtualMachineConfiguration) error {
	if *driver.Yaml.Video.Display == "vz" {
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

func attachAudio(driver *driver.BaseDriver, config *vz.VirtualMachineConfiguration) error {
	if *driver.Yaml.Audio.Device == "vz" {
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
	server := os.NewFile(uintptr(serverFD), "server")
	client := os.NewFile(uintptr(clientFD), "client")
	runtime.SetFinalizer(server, func(file *os.File) {
		logrus.Debugf("Server network file GC'ed")
	})
	runtime.SetFinalizer(client, func(file *os.File) {
		logrus.Debugf("Client network file GC'ed")
	})
	vmNetworkFiles = append(vmNetworkFiles, server, client)
	return server, client, nil
}
