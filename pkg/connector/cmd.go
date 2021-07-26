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

package connector

import (
	"fmt"
	"github.com/fabedge/fabedge/pkg/connector/manager"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

const (
	defaultConfig = "/etc/fabedge/connector.yaml"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "connector",
	Short: "",
	Long:  `connector is part of fabedge, which is responsible for the tunnel/iptables/route management in the cloud.`,
	Run: func(cmd *cobra.Command, args []string) {
		mgr := manager.NewManager()
		mgr.Start()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultConfig, "connector config file")
}

//validateConfig validates the mandatory parameters
// syncPeriod: interval for period sync tasks
// edgePodCIDR: cidr used for pods run in edge
// IP: ip address for connector to terminate tunnels
// Subnets: subnets behind connector
// viciSocket: strongswan vici socket file
// cerFile: X509 cert for connector node
func validateConfig() error {
	allMandatoryKeys := []string{"syncPeriod", "edgePodCIDR", "IP", "Subnets",
		"tunnelConfig", "viciSocket", "certFile", "fabedgeNS"}

	for _, key := range allMandatoryKeys {
		if !viper.IsSet(key) {
			return fmt.Errorf("%s is not set", key)
		}
	}

	return nil
}

func initConfig() {
	// read in connector main config file
	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err != nil {
		klog.Fatalf("failed to parse connector config file: %s", err)
	}

	// read in tunnel config generated in cloud
	tunnelConfig := viper.GetString("tunnelConfig")
	viper.SetConfigFile(tunnelConfig)
	if err := viper.MergeInConfig(); err != nil {
		klog.Fatalf("failed to merge tunnel config file: %s", err)
	}

	viper.AutomaticEnv()

	if err := validateConfig(); err != nil {
		klog.Fatal(err)
	}
}
