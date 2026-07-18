// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cloudinittypes

type UserData struct {
	Growpart *Growpart `yaml:"growpart,omitempty"`

	PackageUpdate           bool `yaml:"package_update,omitempty"`
	PackageUpgrade          bool `yaml:"package_upgrade,omitempty"`
	PackageRebootIfRequired bool `yaml:"package_reboot_if_required,omitempty"`

	Mounts [][]string `yaml:"mounts,omitempty"`

	Timezone string `yaml:"timezone,omitempty"`

	Users []User `yaml:"users,omitempty"`

	WriteFiles []WriteFile `yaml:"write_files,omitempty"`

	ManageResolvConf bool        `yaml:"manage_resolv_conf,omitempty"`
	ResolvConf       *ResolvConf `yaml:"resolv_conf,omitempty"`

	CACerts *CACerts `yaml:"ca_certs,omitempty"`

	BootCmd []string `yaml:"bootcmd,omitempty"`
}

type Growpart struct {
	Mode    string   `yaml:"mode"`
	Devices []string `yaml:"devices"`
}

type User struct {
	Name              string   `yaml:"name,omitempty"`
	Gecos             string   `yaml:"gecos,omitempty"`
	UID               string   `yaml:"uid,omitempty"` // TODO: check if int is allowed too
	Homedir           string   `yaml:"homedir,omitempty"`
	Shell             string   `yaml:"shell,omitempty"`
	Sudo              string   `yaml:"sudo,omitempty"` // TODO: allow []string as well
	LockPasswd        string   `yaml:"lock_passwd,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh-authorized-keys,omitempty"`
}

type WriteFile struct {
	Content     string `yaml:"content"`
	Owner       string `yaml:"owner,omitempty"`
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions,omitempty"` // TODO: check if int is allowed too
}

type ResolvConf struct {
	Nameservers []string `yaml:"nameservers,omitempty"`
}

type CACerts struct {
	RemoveDefaults bool     `yaml:"remove_defaults,omitempty"`
	Trusted        []string `yaml:"trusted,omitempty"`
}
