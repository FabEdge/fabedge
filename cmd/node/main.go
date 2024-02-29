package main

import (
	"github.com/fabedge/fabedge/pkg/agent"
	"github.com/fabedge/fabedge/pkg/cloud-agent"
	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/pkg/connector"
	"github.com/fabedge/fabedge/pkg/operator"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	flag "github.com/spf13/pflag"
	"os"
)

func main() {
	var component string

	fs := flag.CommandLine
	fs.StringVar(&component, "component", "", "the component to initiate")

	// cloud-agent
	var initMembers []string
	fs.StringSliceVar(&initMembers, "connector-node-addresses", []string{}, "internal ip address of all connector nodes")

	// operator
	operatorConfig := &operator.Options{}
	operatorConfig.AddFlags(fs)

	// connector
	connectorConfig := &connector.Config{}
	connectorConfig.AddFlags(fs)

	// agent
	agentConfig := &agent.Config{}
	agentConfig.AddFlags(fs)

	logutil.AddFlags(fs)
	about.AddFlags(fs)

	flag.Parse()

	switch component {
	case "cloud-agent":
		cloud_agent.Execute(initMembers)
	case "operator":
		if err := operator.Execute(operatorConfig); err != nil {
			os.Exit(1)
		}
	case "connector":
		connector.Execute(connectorConfig)
	case "agent":
		if err := agent.Execute(agentConfig); err != nil {
			os.Exit(1)
		}
	}
}
