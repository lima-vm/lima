package sysprof

type SPNetworkDataType struct {
	SPNetworkDataType []NetworkDataType `json:"SPNetworkDataType"`
}

type NetworkDataType struct {
	DNS       DNS    `json:"DNS"`
	Interface string `json:"interface"`
}

type DNS struct {
	ServerAddresses []string `json:"ServerAddresses"`
}
