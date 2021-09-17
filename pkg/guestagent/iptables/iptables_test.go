package iptables

import (
	"strings"
	"testing"
)

// data is from a run of `iptables -t nat -S` with two containers running (started
// with sudo nerdctl) and have exposed ports 8081 and 8082.
const data = `# Warning: iptables-legacy tables present, use iptables-legacy to see them
-P PREROUTING ACCEPT
-P INPUT ACCEPT
-P OUTPUT ACCEPT
-P POSTROUTING ACCEPT
-N CNI-04579c7bb67f4c3f6cca0185
-N CNI-28e04aad9bf52e38b43f8700
-N CNI-2d72aeb202429907277c53c5
-N CNI-2e2f8d5b91929ef9fc152e75
-N CNI-3cbb832b23c724bdddedd7e4
-N CNI-5033e3bad9f1265a2b04037f
-N CNI-DN-04579c7bb67f4c3f6cca0
-N CNI-DN-2d72aeb202429907277c5
-N CNI-DN-2e2f8d5b91929ef9fc152
-N CNI-HOSTPORT-DNAT
-N CNI-HOSTPORT-MASQ
-N CNI-HOSTPORT-SETMARK
-N CNI-cb0db077a14ecd8d4a843636
-N CNI-f1ca917e7b9939c7d8457d68
-A PREROUTING -m addrtype --dst-type LOCAL -j CNI-HOSTPORT-DNAT
-A OUTPUT -m addrtype --dst-type LOCAL -j CNI-HOSTPORT-DNAT
-A POSTROUTING -m comment --comment "CNI portfwd requiring masquerade" -j CNI-HOSTPORT-MASQ
-A POSTROUTING -s 10.4.0.3/32 -m comment --comment "name: \"bridge\" id: \"default-44540a2b2cc6c1154d2a21aec473d6987ec4d6bc339e89ee295a6db433ad623e\"" -j CNI-5033e3bad9f1265a2b04037f
-A POSTROUTING -s 10.4.0.4/32 -m comment --comment "name: \"bridge\" id: \"default-cf12b94944785a4c8937e237a0a277d893cbadebd50409ed5d4b8ca3f90fedf3\"" -j CNI-28e04aad9bf52e38b43f8700
-A POSTROUTING -s 10.4.0.5/32 -m comment --comment "name: \"bridge\" id: \"default-e9d499901490e6a66277688ba8d71cca35a6d1ca6261bc5a7e11e45e80aa3ea3\"" -j CNI-3cbb832b23c724bdddedd7e4
-A POSTROUTING -s 10.4.0.6/32 -m comment --comment "name: \"bridge\" id: \"default-a65e32cc21f9da99b4aa826914873e343f8f09f910657450be551aa24d676e51\"" -j CNI-f1ca917e7b9939c7d8457d68
-A POSTROUTING -s 10.4.0.7/32 -m comment --comment "name: \"bridge\" id: \"default-c93e2a3a2264f98647f0d33dc80d88de81c0710bf30ea822e2ed19213f9c53b5\"" -j CNI-2e2f8d5b91929ef9fc152e75
-A POSTROUTING -s 10.4.0.8/32 -m comment --comment "name: \"bridge\" id: \"default-a8df9868a5f7ee2468118331dd6185e5655f7ff8e77f067408b7ff40e9457860\"" -j CNI-cb0db077a14ecd8d4a843636
-A POSTROUTING -s 10.4.0.9/32 -m comment --comment "name: \"bridge\" id: \"default-393bd750d06186633a02b44487765ce038b7df434bfb16027ca1903bf5f3dc31\"" -j CNI-2d72aeb202429907277c53c5
-A POSTROUTING -s 10.4.0.10/32 -m comment --comment "name: \"bridge\" id: \"default-3d263c6a1c710edc1362764464c073ca834ec9adc0766411772f2b7a3dd1de0f\"" -j CNI-04579c7bb67f4c3f6cca0185
-A CNI-04579c7bb67f4c3f6cca0185 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-3d263c6a1c710edc1362764464c073ca834ec9adc0766411772f2b7a3dd1de0f\"" -j ACCEPT
-A CNI-04579c7bb67f4c3f6cca0185 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-3d263c6a1c710edc1362764464c073ca834ec9adc0766411772f2b7a3dd1de0f\"" -j MASQUERADE
-A CNI-28e04aad9bf52e38b43f8700 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-cf12b94944785a4c8937e237a0a277d893cbadebd50409ed5d4b8ca3f90fedf3\"" -j ACCEPT
-A CNI-28e04aad9bf52e38b43f8700 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-cf12b94944785a4c8937e237a0a277d893cbadebd50409ed5d4b8ca3f90fedf3\"" -j MASQUERADE
-A CNI-2d72aeb202429907277c53c5 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-393bd750d06186633a02b44487765ce038b7df434bfb16027ca1903bf5f3dc31\"" -j ACCEPT
-A CNI-2d72aeb202429907277c53c5 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-393bd750d06186633a02b44487765ce038b7df434bfb16027ca1903bf5f3dc31\"" -j MASQUERADE
-A CNI-2e2f8d5b91929ef9fc152e75 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-c93e2a3a2264f98647f0d33dc80d88de81c0710bf30ea822e2ed19213f9c53b5\"" -j ACCEPT
-A CNI-2e2f8d5b91929ef9fc152e75 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-c93e2a3a2264f98647f0d33dc80d88de81c0710bf30ea822e2ed19213f9c53b5\"" -j MASQUERADE
-A CNI-3cbb832b23c724bdddedd7e4 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-e9d499901490e6a66277688ba8d71cca35a6d1ca6261bc5a7e11e45e80aa3ea3\"" -j ACCEPT
-A CNI-3cbb832b23c724bdddedd7e4 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-e9d499901490e6a66277688ba8d71cca35a6d1ca6261bc5a7e11e45e80aa3ea3\"" -j MASQUERADE
-A CNI-5033e3bad9f1265a2b04037f -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-44540a2b2cc6c1154d2a21aec473d6987ec4d6bc339e89ee295a6db433ad623e\"" -j ACCEPT
-A CNI-5033e3bad9f1265a2b04037f ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-44540a2b2cc6c1154d2a21aec473d6987ec4d6bc339e89ee295a6db433ad623e\"" -j MASQUERADE
-A CNI-DN-04579c7bb67f4c3f6cca0 -s 10.4.0.0/24 -p tcp -m tcp --dport 8082 -j CNI-HOSTPORT-SETMARK
-A CNI-DN-04579c7bb67f4c3f6cca0 -s 127.0.0.1/32 -p tcp -m tcp --dport 8082 -j CNI-HOSTPORT-SETMARK
-A CNI-DN-04579c7bb67f4c3f6cca0 -p tcp -m tcp --dport 8082 -j DNAT --to-destination 10.4.0.10:80
-A CNI-DN-2e2f8d5b91929ef9fc152 -s 10.4.0.0/24 -d 127.0.0.1/32 -p tcp -m tcp --dport 8081 -j CNI-HOSTPORT-SETMARK
-A CNI-DN-2e2f8d5b91929ef9fc152 -s 127.0.0.1/32 -d 127.0.0.1/32 -p tcp -m tcp --dport 8081 -j CNI-HOSTPORT-SETMARK
-A CNI-DN-2e2f8d5b91929ef9fc152 -d 127.0.0.1/32 -p tcp -m tcp --dport 8081 -j DNAT --to-destination 10.4.0.7:80
-A CNI-HOSTPORT-DNAT -p tcp -m comment --comment "dnat name: \"bridge\" id: \"default-c93e2a3a2264f98647f0d33dc80d88de81c0710bf30ea822e2ed19213f9c53b5\"" -m multiport --dports 8081 -j CNI-DN-2e2f8d5b91929ef9fc152
-A CNI-HOSTPORT-DNAT -p tcp -m comment --comment "dnat name: \"bridge\" id: \"default-393bd750d06186633a02b44487765ce038b7df434bfb16027ca1903bf5f3dc31\"" -m multiport --dports 8082 -j CNI-DN-2d72aeb202429907277c5
-A CNI-HOSTPORT-DNAT -p tcp -m comment --comment "dnat name: \"bridge\" id: \"default-3d263c6a1c710edc1362764464c073ca834ec9adc0766411772f2b7a3dd1de0f\"" -m multiport --dports 8082 -j CNI-DN-04579c7bb67f4c3f6cca0
-A CNI-HOSTPORT-MASQ -m mark --mark 0x2000/0x2000 -j MASQUERADE
-A CNI-HOSTPORT-SETMARK -m comment --comment "CNI portfwd masquerade mark" -j MARK --set-xmark 0x2000/0x2000
-A CNI-cb0db077a14ecd8d4a843636 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-a8df9868a5f7ee2468118331dd6185e5655f7ff8e77f067408b7ff40e9457860\"" -j ACCEPT
-A CNI-cb0db077a14ecd8d4a843636 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-a8df9868a5f7ee2468118331dd6185e5655f7ff8e77f067408b7ff40e9457860\"" -j MASQUERADE
-A CNI-f1ca917e7b9939c7d8457d68 -d 10.4.0.0/24 -m comment --comment "name: \"bridge\" id: \"default-a65e32cc21f9da99b4aa826914873e343f8f09f910657450be551aa24d676e51\"" -j ACCEPT
-A CNI-f1ca917e7b9939c7d8457d68 ! -d 224.0.0.0/4 -m comment --comment "name: \"bridge\" id: \"default-a65e32cc21f9da99b4aa826914873e343f8f09f910657450be551aa24d676e51\"" -j MASQUERADE
`

func TestParsePortsFromRules(t *testing.T) {

	// Turn the string into individual lines
	rules := strings.Split(data, "\n")
	if len(rules) > 0 && rules[len(rules)-1] == "" {
		rules = rules[:len(rules)-1]
	}

	res, err := parsePortsFromRules(rules)
	if err != nil {
		t.Errorf("parsing iptables ports failed with error: %s", err)
	}

	l := len(res)
	if l != 2 {
		t.Fatalf("expected 2 ports parsed from iptables but parsed %d", l)
	}

	if res[0].IP.String() != "127.0.0.1" || res[0].Port != 8082 || res[0].TCP != true {
		t.Errorf("expected port 8082 on IP 127.0.0.1 with TCP true but go port %d on IP %s with TCP %t", res[0].Port, res[0].IP.String(), res[0].TCP)
	}
	if res[1].IP.String() != "127.0.0.1" || res[1].Port != 8081 || res[1].TCP != true {
		t.Errorf("expected port 8081 on IP 127.0.0.1 with TCP true but go port %d on IP %s with TCP %t", res[1].Port, res[1].IP.String(), res[1].TCP)
	}
}
