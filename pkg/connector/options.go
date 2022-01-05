package connector

import (
	"github.com/spf13/pflag"
	"time"
)

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.TunnelConfigFile, "tunnel-config", "/etc/fabedge/tunnels.yaml", "tunnel config file")
	fs.StringVar(&c.CertFile, "cert-file", "/etc/ipsec.d/certs/tls.crt", "TLS certificate file")
	fs.StringVar(&c.ViciSocket, "vici-socket", "/var/run/charon.vici", "vici socket file")
	fs.StringVar(&c.CNIType, "cni-type", "flannel", "CNI type used in cloud")
	fs.DurationVar(&c.SyncPeriod, "sync-period", 5*time.Minute, "period to sync routes/rules")
	fs.DurationVar(&c.DebounceDuration, "debounce-duration", 5*time.Second, "period to sync routes/rules")
	fs.StringSliceVar(&c.initMembers, "connector-node-addresses", []string{}, "internal address of all connector nodes")
}
