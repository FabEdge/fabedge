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

package agent

import (
	"fmt"
	"os"

	"github.com/coredns/coredns/coremain"
	"github.com/fsnotify/fsnotify"
	flag "github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
)

func Execute(cfg *Config) error {
	defer klog.Flush()

	// the version flag is added by importing kube-proxy packages,
	// but I don't know how that happened
	if flag.Lookup("version").Value.String() == "true" {
		about.DisplayVersion()
		fmt.Println("----------------")
		fmt.Println("kube-proxy: 1.22.5")
		fmt.Println("----------------")
		coremain.Run()
	}

	log := klogr.New().WithName("manager")
	if err := cfg.Validate(); err != nil {
		log.Error(err, "validation failed")
		return err
	}

	manager, err := cfg.Manager()
	if err != nil {
		log.Error(err, "failed to create manager")
		return err
	}

	if err := os.MkdirAll(cfg.CNI.ConfDir, 0777); err != nil {
		log.Error(err, "failed to create cni conf dir")
		return err
	}

	if err := os.MkdirAll(cfg.Workdir, 0777); err != nil {
		log.Error(err, "failed to create cni conf dir")
		return err
	}

	go manager.start()

	err = watchTunnelConfigFile(cfg.TunnelsConfPath, func(event fsnotify.Event) {
		log.V(5).Info("tunnels or services config may change", "file", event.Name, "event", event.Op.String())
		manager.notify()
	})
	if err != nil {
		log.Error(err, "failed to watch tunnels config file", "file", cfg.TunnelsConfPath)
		return err
	}

	return nil
}

func watchTunnelConfigFile(tunnelsConfpath string, handleFn func(event fsnotify.Event)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err = watcher.Add(tunnelsConfpath); err != nil {
		return err
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			handleFn(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}
