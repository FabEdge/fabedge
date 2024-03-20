package main

import (
	"fmt"
	"github.com/fabedge/fabedge/pkg/agent"
	"github.com/fabedge/fabedge/pkg/cloud-agent"
	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/pkg/connector"
	"github.com/fabedge/fabedge/pkg/operator"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	flag "github.com/spf13/pflag"
	"os"
	"time"
)

func main() {
	var component string

	fs := flag.CommandLine
	fs.StringVar(&component, "component", "", "the component to initiate")

	// common
	var cniType string
	var leaseDuration time.Duration
	var renewDeadline time.Duration
	var retryPeriod time.Duration
	var syncPeriod time.Duration
	var tunnelInitTimeout uint

	// cloud-agent
	var initMembers []string
	fs.StringSliceVar(&initMembers, "connector-node-addresses", []string{}, "internal ip address of all connector nodes")

	// common
	fs.StringVar(&cniType, "cni-type", "", "The CNI name in your kubernetes cluster")
	fs.DurationVar(&leaseDuration, "leader-lease-duration", 15*time.Second, "The duration that non-leader candidates will wait to force acquire leadership")
	fs.DurationVar(&renewDeadline, "leader-renew-deadline", 10*time.Second, "The duration that the acting controlplane will retry refreshing leadership before giving up")
	fs.DurationVar(&retryPeriod, "leader-retry-period", 2*time.Second, "The duration that the LeaderElector clients should wait between tries of actions")
	fs.UintVar(&tunnelInitTimeout, "tunnel-init-timeout", 10, "The timeout of tunnel initiation. Uint: second")
	// agent: 30s, connector: 5m
	fs.DurationVar(&syncPeriod, "sync-period", 30*time.Second, "The period to synchronize network configuration")

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

	fmt.Printf("component: %s\n", component)

	switch component {
	case "cloud-agent":
		cloud_agent.Execute(initMembers)
	case "operator":
		operatorConfig.CNIType = cniType
		operatorConfig.ManagerOpts.LeaseDuration = &leaseDuration
		operatorConfig.ManagerOpts.RenewDeadline = &renewDeadline
		operatorConfig.ManagerOpts.RetryPeriod = &retryPeriod
		if err := operator.Execute(operatorConfig); err != nil {
			os.Exit(1)
		}
	case "connector":
		connectorConfig.CNIType = cniType
		connectorConfig.SyncPeriod = syncPeriod
		connectorConfig.InitMembers = initMembers
		connectorConfig.LeaderElection.LeaseDuration = leaseDuration
		connectorConfig.LeaderElection.RenewDeadline = renewDeadline
		connectorConfig.LeaderElection.RetryPeriod = retryPeriod
		connectorConfig.TunnelInitTimeout = tunnelInitTimeout
		connector.Execute(connectorConfig)
	case "agent":
		agentConfig.SyncPeriod = syncPeriod
		agentConfig.TunnelInitTimeout = tunnelInitTimeout
		if err := agent.Execute(agentConfig); err != nil {
			os.Exit(1)
		}
	}
}
