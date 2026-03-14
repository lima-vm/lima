// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// From https://raw.githubusercontent.com/abiosoft/colima/v0.5.5/daemon/process/gvproxy/dnshosts_test.go
/*
	MIT License

	Copyright (c) 2021 Abiola Ibrahim

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/

package dnshosts

import (
	"net"
	"strings"

	"github.com/containers/gvisor-tap-vsock/pkg/types"
)

func ExtractZones(hosts hostMap) []types.Zone {
	list := make(map[string]types.Zone)

	for host := range hosts {
		h := zoneHost(host)

		zone := types.Zone{Name: h.name()}
		if existingZone, ok := list[h.name()]; ok {
			zone = existingZone
		}

		if h.recordName() == "" {
			if zone.DefaultIP == nil {
				zone.DefaultIP = hosts.hostIP(host)
			}
		} else {
			zone.Records = append(zone.Records, types.Record{
				Name: h.recordName(),
				IP:   hosts.hostIP(host),
			})
		}

		list[h.name()] = zone
	}

	zones := make([]types.Zone, 0, len(list))
	for _, zone := range list {
		zones = append(zones, zone)
	}
	return zones
}

type hostMap map[string]string

func (z hostMap) hostIP(host string) net.IP {
	for {
		// check if host entry exists
		h, ok := z[host]
		if !ok || h == "" {
			return nil
		}

		// if it's a valid ip, return
		if ip := net.ParseIP(h); ip != nil {
			return ip
		}

		// otherwise, a string i.e. another host
		// loop through the process again.
		host = h
	}
}

type zoneHost string

func (z zoneHost) name() string {
	i := z.dotIndex()
	if i < 0 {
		return string(z)
	}
	return string(z)[i+1:] + "."
}

func (z zoneHost) recordName() string {
	i := z.dotIndex()
	if i < 0 {
		return ""
	}
	return string(z)[:i]
}

func (z zoneHost) dotIndex() int {
	return strings.LastIndex(string(z), ".")
}
