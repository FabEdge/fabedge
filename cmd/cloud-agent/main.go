package main

import (
	"github.com/fabedge/fabedge/pkg/cloud-agent"
	"github.com/fabedge/fabedge/pkg/common/about"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	flag "github.com/spf13/pflag"
)

func main() {
	var initMembers []string

	flag.StringSliceVar(&initMembers, "connector-node-addresses", []string{}, "internal ip address of all connector nodes")
	logutil.AddFlags(flag.CommandLine)
	about.AddFlags(flag.CommandLine)

	flag.Parse()

	cloud_agent.Execute(initMembers)
}
