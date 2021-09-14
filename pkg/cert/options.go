// Copyright 2021 FabEdge Team
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
	"crypto/x509"
	"fmt"
	"net"

	flag "github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/client"

	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
)

type GlobalOptions struct {
	Namespace string

	CASecret string
	CACert   string
	CAKey    string
}

func (opts *GlobalOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVarP(&opts.Namespace, "namespace", "n", "fabedge", "The namespace to save secret or get secret")
	fs.StringVar(&opts.CASecret, "ca-secret", "fabedge-ca", "The name of ca secret, the CLI by default read CA cert/key from secret")
	fs.StringVar(&opts.CACert, "ca-cert", "", "The CA cert filename, provide it if you prefer file")
	fs.StringVar(&opts.CAKey, "ca-key", "", "The CA cert key filename, provide it if you prefer file")
}

func (opts *GlobalOptions) CAIsFromSecret() bool {
	return len(opts.CACert) == 0 || len(opts.CAKey) == 0
}

func (opts *GlobalOptions) SecretKey() client.ObjectKey {
	return client.ObjectKey{
		Name:      opts.CASecret,
		Namespace: opts.Namespace,
	}
}

type SaveOptions struct {
	Name   string
	Secret bool
	File   bool
	// todo: force flag
}

func (opts *SaveOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&opts.Name, "secret-name", "", "The name of the secret to save, if not provided, the commonName will be used(not work on CA)")
	fs.BoolVar(&opts.Secret, "save-to-secret", true, "Save cert/key into secret")
	fs.BoolVar(&opts.File, "save-to-file", false, "Save cert/key to files")
}

type CertOptions struct {
	Organization   []string
	ValidityPeriod int64
	IPs            []string
	DNSNames       []string
}

func (opts *CertOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringSliceVarP(&opts.Organization, "organization", "O", []string{certutil.DefaultOrganization}, "your organization name")
	fs.Int64Var(&opts.ValidityPeriod, "validity-period", 365, "validity period for your cert, unit: day")
	fs.StringSliceVar(&opts.IPs, "ips", nil, "The ip addresses for your cert, e.g. 2.2.2.2,10.10.10.10")
	fs.StringSliceVar(&opts.DNSNames, "dns-names", nil, "The dns names for your cert, e.g. fabedge.io,yourdomain.com")
}

func (opts *CertOptions) AsConfig(cn string, isCA bool, usages []x509.ExtKeyUsage) certutil.Config {
	return certutil.Config{
		CommonName:     cn,
		IsCA:           isCA,
		Organization:   opts.Organization,
		IPs:            opts.GetIPs(),
		DNSNames:       opts.DNSNames,
		ValidityPeriod: timeutil.Days(opts.ValidityPeriod),
		Usages:         usages,
	}
}

func (opts *CertOptions) Validate() error {
	for _, v := range opts.IPs {
		if net.ParseIP(v) == nil {
			return fmt.Errorf("invalid ip: %s", v)
		}
	}

	return nil
}

func (opts *CertOptions) GetIPs() (ips []net.IP) {
	for _, v := range opts.IPs {
		ips = append(ips, net.ParseIP(v))
	}

	return ips
}

type VerifyOptions struct {
	Secret string
	File   string
}

func (opts *VerifyOptions) AddFlags(fs *flag.FlagSet) {
	fs.StringVarP(&opts.Secret, "secret", "s", "", "your cert secret name")
	fs.StringVarP(&opts.File, "file", "f", "", "your cert file name")
}

func (opts *VerifyOptions) Validate() error {
	if len(opts.Secret) == 0 && len(opts.File) == 0 {
		return fmt.Errorf("you must specify a secret name or filename of your cert")
	}

	return nil
}
