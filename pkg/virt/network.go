package virt

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/lima-vm/lima/pkg/driver"
)

type NetworkConfig struct {
	Name    string
	Address string
	Netmask string
}

const networkXMLTemplate = `<network>
  <name>{{.Name}}</name>
  <ip address='{{.Address}}' netmask='{{.Netmask}}'>
  </ip>
</network>`

func NetworkXML(driver *driver.BaseDriver) (string, error) {
	tmpl, err := template.New("network").Parse(networkXMLTemplate)
	if err != nil {
		return "", err
	}
	cfg := NetworkConfig{
		Name:    "lima-" + driver.Instance.Name,
		Address: "192.168.55.15",
		Netmask: "255.255.255.0",
	}
	var xml bytes.Buffer
	err = tmpl.Execute(&xml, cfg)
	if err != nil {
		return "", err
	}
	networkXML := filepath.Join(driver.Instance.Dir, "network.xml")
	err = os.WriteFile(networkXML, xml.Bytes(), 0o644)
	if err != nil {
		return "", err
	}
	return networkXML, nil
}
