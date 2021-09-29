package sysprof

type SPNetworkDataType struct {
	SPNetworkDataType []NetworkDataType `json:"SPNetworkDataType"`
}

type NetworkDataType struct {
	DNS       DNS     `json:"DNS"`
	Interface string  `json:"interface"`
	Proxies   Proxies `json:"Proxies"`
}

type DNS struct {
	ServerAddresses []string `json:"ServerAddresses"`
}

type Proxies struct {
	ExceptionList []string `json:"ExceptionList"` // default: ["*.local", "169.254/16"]
	FTPEnable     string   `json:"FTPEnable"`
	FTPPort       int      `json:"FTPPort"`
	FTPProxy      string   `json:"FTPProxy"`
	FTPUser       string   `json:"FTPUser"`
	HTTPEnable    string   `json:"HTTPEnable"`
	HTTPPort      int      `json:"HTTPPort"`
	HTTPProxy     string   `json:"HTTPProxy"`
	HTTPUser      string   `json:"HTTPUser"`
	HTTPSEnable   string   `json:"HTTPSEnable"`
	HTTPSPort     int      `json:"HTTPSPort"`
	HTTPSProxy    string   `json:"HTTPSProxy"`
	HTTPSUser     string   `json:"HTTPSUser"`
}
