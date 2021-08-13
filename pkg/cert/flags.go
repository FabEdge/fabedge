// Copyright 2021 BoCloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cert

import (
	"flag"
	"fmt"
	"net"
	"strings"
)

var (
	version bool

	kubeConfig string
	namespace  string

	commonName     string
	validityPeriod int

	ipAddresses string
	dnsNames    string

	signConnectorCert bool
	signEdgeCert      bool

	saveToFile bool
)

func init() {
	flag.BoolVar(&version, "version", false, "display version info")

	flag.StringVar(&kubeConfig, "kubeconfig", "/root/.kube/config", "Path to the kubeconfig file to use for CLI requests")
	flag.StringVar(&namespace, "namespace", "fabedge", "The namespace in which ca-certs secret and connector-certs secret will be created")

	flag.StringVar(&commonName, "common-name", "", "Common Name of the certificate subject")
	flag.IntVar(&validityPeriod, "validity-period", 3650, "Validity period of the certificate, unit is the day")

	flag.StringVar(&ipAddresses, "ip-addresses", "", "Subject Alternate Name values of the certificate. When there are multiple values, use commas to join them")
	flag.StringVar(&dnsNames, "dns-names", "", "Subject Alternate Name values of the certificate. When there are multiple values, use commas to join them")

	flag.BoolVar(&signConnectorCert, "sign-connector-cert", false, "Issue certificate for connector")
	flag.BoolVar(&signEdgeCert, "sign-edge-cert", false, "Issue certificate for edge")
	flag.BoolVar(&saveToFile, "save-to-file", false, "Whether to save the certificate to a file")
}

func validateFlags() error {
	if len(commonName) == 0 {
		return fmt.Errorf("the value of -common-name can't be empty")
	}

	if validityPeriod <= 0 {
		return fmt.Errorf("the least validity period value is 1")
	}

	if signConnectorCert && signEdgeCert {
		return fmt.Errorf("only one certificate can be issued at a time")
	}

	if !signConnectorCert && !signEdgeCert {
		return fmt.Errorf("at least one of -sign-connector-cert and -sign-edge-cert should be true")
	}

	if signEdgeCert && commonName == connectorCommonName {
		return fmt.Errorf("when -sign-edge-cert is true, -common-name value cannot be equal to connector")
	}
	if len(ipAddresses) != 0 {
		ipAddrs := strings.Split(ipAddresses, ",")
		for _, addr := range ipAddrs {
			ip := net.ParseIP(addr)
			if ip == nil {
				return fmt.Errorf("invalid IP address %s", addr)
			}
		}
	}
	return nil
}

func parseIPAddressesFlags() []net.IP {
	if len(ipAddresses) == 0 {
		return nil
	}

	ipAddrs := strings.Split(ipAddresses, ",")
	ips := []net.IP{}
	for _, ip := range ipAddrs {
		ips = append(ips, net.ParseIP(ip))
	}
	return ips
}

func parseDNSNamesFlags() []string {
	if signConnectorCert {
		commonName = connectorCommonName
	}

	// dnsNames default value is commonName
	if len(dnsNames) == 0 {
		return []string{commonName}
	}

	names := strings.Split(dnsNames, ",")
	dns := []string{}
	dns = append(dns, names...)
	return dns
}

func parseCommonNameFlags() string {
	if signConnectorCert {
		commonName = connectorCommonName
	}
	return commonName
}
