package main

import (
	"libvirt.org/go/libvirt"
)

var VERSION uint32 = libvirt.VERSION_NUMBER

func Version() (uint32, error) { return libvirt.GetVersion() }

const connectString = "qemu:///session"

func CreateNetwork(xml string) error {
	conn, err := libvirt.NewConnect(connectString)
	if err != nil {
		return err
	}
	defer conn.Close()
	net, err := conn.NetworkDefineXML(xml)
	if err != nil {
		return err
	}
	return net.Create()
}

func CreateDomain(xml string) error {
	conn, err := libvirt.NewConnect(connectString)
	if err != nil {
		return err
	}
	defer conn.Close()
	vm, err := conn.DomainDefineXML(xml)
	if err != nil {
		return err
	}
	return vm.Create()
}
