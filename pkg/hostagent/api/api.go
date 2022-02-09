package api

type Info struct {
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}

type Status struct {
	Running bool `json:"running"`
	Paused  bool `json:"paused"`
}
