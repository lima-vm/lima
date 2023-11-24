package main

import (
	"libvirt.org/go/libvirt"
)

var VERSION uint32 = libvirt.VERSION_NUMBER

func Version() (uint32, error) { return libvirt.GetVersion() }
