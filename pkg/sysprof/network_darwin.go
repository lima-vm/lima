package sysprof

type SPNetworkDataType struct {
	SPNetworkDataType []NetworkDataType `json:"SPNetworkDataType"`
}

type NetworkDataType struct {
	DNS       DNS     `json:"DNS"`
	Interface string  `json:"interface"`
	IPv4      IPv4    `json:"IPv4,omitempty"`
	Proxies   Proxies `json:"Proxies"`
}

type DNS struct {
	ServerAddresses []string `json:"ServerAddresses"`
}

type IPv4 struct {
	Addresses []string `json:"Addresses,omitempty"`
}

type Proxies struct {
	ExceptionList []string `json:"ExceptionList"` // default: ["*.local", "169.254/16"]
	FTPEnable     string   `json:"FTPEnable"`
	FTPPort       any      `json:"FTPPort"`
	FTPProxy      string   `json:"FTPProxy"`
	FTPUser       string   `json:"FTPUser"`
	HTTPEnable    string   `json:"HTTPEnable"`
	HTTPPort      any      `json:"HTTPPort"`
	HTTPProxy     string   `json:"HTTPProxy"`
	HTTPUser      string   `json:"HTTPUser"`
	HTTPSEnable   string   `json:"HTTPSEnable"`
	HTTPSPort     any      `json:"HTTPSPort"`
	HTTPSProxy    string   `json:"HTTPSProxy"`
	HTTPSUser     string   `json:"HTTPSUser"`
}
