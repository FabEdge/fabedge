package log

import (
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func AddFlags(fs *pflag.FlagSet) {
	local := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(local)

	fs.AddGoFlag(local.Lookup("v"))
}
