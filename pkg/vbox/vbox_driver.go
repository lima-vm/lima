package vbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

type LimaVBoxDriver struct {
	*driver.BaseDriver
	qCmd    *exec.Cmd
	qWaitCh chan error
}

func New(driver *driver.BaseDriver) *LimaVBoxDriver {
	return &LimaVBoxDriver{
		BaseDriver: driver,
	}
}

func (l *LimaVBoxDriver) Validate() error {
	if *l.Instance.Config.Arch != limayaml.X8664 {
		return fmt.Errorf("field `arch` must be %q for VBox driver , got %q", limayaml.X8664, *l.Instance.Config.Arch)
	}
	if *l.Instance.Config.MountType != limayaml.REVSSHFS {
		return fmt.Errorf("field `mountType` must be %q for VBox driver , got %q", limayaml.REVSSHFS, *l.Instance.Config.MountType)
	}
	return nil
}

func (l *LimaVBoxDriver) CreateDisk(ctx context.Context) error {
	return EnsureDisk(ctx, l.BaseDriver)
}

func (l *LimaVBoxDriver) create(ctx context.Context, name string) error {
	qExe := "VBoxManage"

	qArgsFinal := []string{"createvm", "--basefolder", l.Instance.Dir, "--name", name, "--register"}
	qCmd := exec.CommandContext(ctx, qExe, qArgsFinal...)
	_, err := qCmd.StdoutPipe()
	if err != nil {
		return err
	}
	logrus.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Run(); err != nil {
		return err
	}

	baseDisk := filepath.Join(l.Instance.Dir, filenames.BaseDisk)
	diffDisk := filepath.Join(l.Instance.Dir, filenames.DiffDisk)
	extraDisks := []string{}
	if len(l.Instance.AdditionalDisks) > 0 {
		for _, d := range l.Instance.AdditionalDisks {
			diskName := d.Name
			disk, err := store.InspectDisk(diskName)
			if err != nil {
				logrus.Errorf("could not load disk %q: %q", diskName, err)
				return err
			}

			if disk.Instance != "" {
				logrus.Errorf("could not attach disk %q, in use by instance %q", diskName, disk.Instance)
				return err
			}
			logrus.Infof("Mounting disk %q on %q", diskName, disk.MountPoint)
			err = disk.Lock(l.Instance.Dir)
			if err != nil {
				logrus.Errorf("could not lock disk %q: %q", diskName, err)
				return err
			}
			dataDisk := filepath.Join(disk.Dir, filenames.DataDisk)
			extraDisks = append(extraDisks, dataDisk)
		}
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}

	var firmware string
	if *l.Instance.Config.Firmware.LegacyBIOS {
		firmware = "bios"
	} else {
		firmware = "efi"
	}
	cpus := *l.Instance.Config.CPUs
	memBytes, err := units.RAMInBytes(*l.Instance.Config.Memory)
	if err != nil {
		return err
	}
	memory := memBytes >> 20
	var boot string
	if isBaseDiskISO {
		boot = "dvd"
	} else {
		boot = "disk"
	}

	modifyFlags := []string{
		"modifyvm", name,
		"--firmware", firmware,
		"--ostype", "Linux26_64",
		"--cpus", fmt.Sprintf("%d", cpus),
		"--memory", fmt.Sprintf("%d", memory),
		"--boot1", boot,
	}
	mCmd := exec.CommandContext(ctx, qExe, modifyFlags...)
	logrus.Debugf("mCmd.Args: %v", mCmd.Args)
	if err := mCmd.Run(); err != nil {
		return err
	}

	logrus.Debugf("storage")
	if err := exec.CommandContext(ctx, qExe, "storagectl", name,
		"--name", "SATA",
		"--add", "sata",
		"--portcount", "4",
		"--hostiocache", "on").Run(); err != nil {
		logrus.Debugf("storagectl %v", err)
		return err
	}
	if isBaseDiskISO {
		if err := exec.CommandContext(ctx, qExe, "storageattach", name,
			"--storagectl", "SATA",
			"--port", "1",
			"--device", "0",
			"--type", "dvddrive",
			"--medium", baseDisk+".iso").Run(); err != nil {
			logrus.Debugf("basedisk %v", err)
			return err
		}
	}
	if err := exec.CommandContext(ctx, qExe, "storageattach", name,
		"--storagectl", "SATA",
		"--port", "0",
		"--device", "0",
		"--type", "hdd",
		"--medium", diffDisk+".vdi").Run(); err != nil {
		logrus.Debugf("diffdisk %v", err)
		return err
	}
	for i, extraDisk := range extraDisks {
		if err := exec.CommandContext(ctx, qExe, "storageattach", name,
			"--storagectl", "SATA",
			"--port", "3",
			"--device", fmt.Sprintf("%d", i),
			"--type", "hdd",
			"--medium", extraDisk).Run(); err != nil {
			logrus.Debugf("extradisk %v", err)
			return err
		}
	}

	if err := exec.CommandContext(ctx, qExe, "storageattach", name,
		"--storagectl", "SATA",
		"--type", "dvddrive",
		"--port", "2",
		"--device", "0",
		"--medium", filepath.Join(l.Instance.Dir, filenames.CIDataISO)).Run(); err != nil {
		logrus.Debugf("cidata %v", err)
		return err
	}

	logrus.Debugf("network")

	slirpMACAddress := limayaml.MACAddress(l.Instance.Dir)
	if out, err := exec.CommandContext(ctx, qExe, "modifyvm", name,
		"--nic1", "nat",
		"--macaddress1", strings.ReplaceAll(slirpMACAddress, ":", ""),
		"--nictype1", "virtio",
		"--cableconnected1", "on").CombinedOutput(); err != nil {
		logrus.Debugf("modifyvm nic1 %v %s", err, out)
		return err
	}

	return nil
}

func (l *LimaVBoxDriver) Start(ctx context.Context) (chan error, error) {
	name := "lima-" + l.Instance.Name
	qExe := "VBoxManage"
	if err := exec.CommandContext(ctx, qExe, "showvminfo", name).Run(); err != nil {
		err = l.create(ctx, name)
		if err != nil {
			return nil, err
		}
	}

	_ = exec.CommandContext(ctx, qExe, "modifyvm", name,
		"--natpf1", "delete", "ssh").Run()
	if out, err := exec.CommandContext(ctx, qExe, "modifyvm", name,
		"--natpf1", fmt.Sprintf("%s,%s,127.0.0.1,%d,,%d", "ssh", "tcp", l.SSHLocalPort, 22)).CombinedOutput(); err != nil {
		logrus.Debugf("modifyvm natpf1 %v %s", err, out)
		return nil, err
	}
	if out, err := exec.CommandContext(ctx, qExe, "modifyvm", name,
		"--uart1", "0x3F8", "4", // these are the "traditional values" for COM1, according to the documentation
		"--uartmode1", "file", filepath.Join(l.Instance.Dir, filenames.SerialLog)).CombinedOutput(); err != nil {
		logrus.Debugf("modifyvm uartmode %v %s", err, out)
		return nil, err
	}

	logrus.Infof("Starting VBox (hint: to watch the boot progress, see %q)", filepath.Join(l.Instance.Dir, filenames.SerialLog))
	displayType := "headless"
	if l.Instance.Config.Video.Display != nil {
		display := *l.Instance.Config.Video.Display
		if display == "none" || display == "headless" {
			displayType = "headless"
		} else { // display == "default" || display == "gui"
			displayType = "gui"
		}
	}
	startFlags := []string{
		"startvm", name,
		"--type", displayType,
	}
	qCmd := exec.CommandContext(ctx, qExe, startFlags...)
	_, err := qCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	logrus.Debugf("qCmd.Args: %v", qCmd.Args)
	if err := qCmd.Start(); err != nil {
		logrus.Debugf("%v", err)
		return nil, err
	}

	l.qCmd = qCmd
	l.qWaitCh = make(chan error)
	go func() {
		for {
			time.Sleep(1 * time.Second)
		}
	}()
	logrus.Info("Started VBox")

	// TODO: get Pid of VM
	pidFile := filepath.Join(l.Instance.Dir, filenames.PIDFile(*l.Instance.Config.VMType))
	if _, err := os.Stat(pidFile); !errors.Is(err, os.ErrNotExist) {
		logrus.Errorf("pidfile %q already exists", pidFile)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		logrus.Errorf("error writing to pid file %q", pidFile)
	}

	return l.qWaitCh, nil
}

func (l *LimaVBoxDriver) Stop(ctx context.Context) error {
	args := []string{"controlvm", "lima-" + l.Instance.Name, "acpipowerbutton"}
	qCmd := exec.CommandContext(ctx, "VBoxManage", args...)
	err := qCmd.Run()
	return err
}

func (l *LimaVBoxDriver) Register(ctx context.Context) error {
	name := "lima-" + l.Instance.Name
	args := []string{"registervm", filepath.Join(l.Instance.Dir, name)}
	qCmd := exec.CommandContext(ctx, "VBoxManage", args...)
	err := qCmd.Run()
	return err
}

func (l *LimaVBoxDriver) Unregister(ctx context.Context) error {
	args := []string{"unregistervm", "lima-" + l.Instance.Name}
	qCmd := exec.CommandContext(ctx, "VBoxManage", args...)
	err := qCmd.Run()
	return err
}
