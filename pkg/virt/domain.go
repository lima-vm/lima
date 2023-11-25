package virt

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/lima-vm/lima/pkg/driver"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type DomainConfig struct {
	Name            string
	Memory          int64
	CPUs            int
	CIDataPath      string
	DiskPath        string
	Interface       string
	QemuCommandline string
	Network         string
	SlirpNetwork    string
	SlirpIPAddress  string
	SSHLocalPort    int
}

const domainXMLTemplate = `<domain type='kvm' xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'>
  <name>{{.Name}}</name><memory unit='B'>{{.Memory}}</memory>
  <vcpu>{{.CPUs}}</vcpu>
  <features><acpi/><apic/></features>
  <cpu mode='host-passthrough'></cpu>
  <os firmware='efi'>
    <type>hvm</type>
    <boot dev='cdrom'/>
    <boot dev='hd'/>
    <bootmenu enable='no'/>
  </os>
  <devices>
    <console type='pty'>
      <target type='serial'/>
    </console>
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='{{.CIDataPath}}'/>
      <target dev='hdd' bus='sata'/>
      <readonly/>
    </disk>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='{{.DiskPath}}'/>
      <target dev='vda' bus='virtio'/>
    </disk>
<!--
    <graphics type='vnc' autoport='yes' listen='127.0.0.1'>
      <listen type='address' address='127.0.0.1'/>
    </graphics>
-->
{{.Interface}}
  </devices>
{{.QemuCommandline}}
</domain>`

const interfaceXMLTemplate = `
    <interface type='user'>
      <source network='{{.Network}}'/>
      <model type='virtio'/>
    </interface>`

const qemuXMLTemplate = `
    <qemu:commandline>
        <qemu:arg value='-netdev'/>
        <qemu:arg value='user,id=net0,net={{.SlirpNetwork}},dhcpstart={{.SlirpIPAddress}},hostfwd=tcp:127.0.0.1:{{.SSHLocalPort}}-:22'/>
        <qemu:arg value='-device'/>
        <qemu:arg value='virtio-net-pci,netdev=net0'/>
        <qemu:arg value='-device'/>
        <qemu:arg value='virtio-rng-pci'/>
    </qemu:commandline>`

func DomainXML(driver *driver.BaseDriver) (string, error) {
	tmpl, err := template.New("domain").Parse(domainXMLTemplate)
	if err != nil {
		return "", err
	}
	qemu, err := template.New("qemu").Parse(qemuXMLTemplate)
	if err != nil {
		return "", err
	}
	//baseDisk := filepath.Join(driver.Instance.Dir, filenames.BaseDisk)
	diffDisk := filepath.Join(driver.Instance.Dir, filenames.DiffDisk)
	ciDataPath := filepath.Join(driver.Instance.Dir, filenames.CIDataISO)
	cfg := DomainConfig{
		Name:           "lima-" + driver.Instance.Name,
		Memory:         driver.Instance.Memory,
		CPUs:           driver.Instance.CPUs,
		CIDataPath:     ciDataPath,
		DiskPath:       diffDisk,
		//Network:        "lima-" + driver.Instance.Name,
		SlirpNetwork:   networks.SlirpNetwork,
		SlirpIPAddress: networks.SlirpIPAddress,
		SSHLocalPort:   driver.SSHLocalPort,
	}
	var buf bytes.Buffer
	err = qemu.Execute(&buf, cfg)
	if err != nil {
		return "", err
	}
	cfg.QemuCommandline = buf.String()

	var xml bytes.Buffer
	err = tmpl.Execute(&xml, cfg)
	if err != nil {
		return "", err
	}
	domainXML := filepath.Join(driver.Instance.Dir, "domain.xml")
	err = os.WriteFile(domainXML, xml.Bytes(), 0o644)
	if err != nil {
		return "", err
	}
	return xml.String(), nil
}
