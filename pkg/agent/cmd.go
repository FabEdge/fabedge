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
	"flag"
	"os"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
)

func Execute() error {
	klog.InitFlags(nil)
	// init klog level
	_ = flag.Set("v", "3")
	flag.Parse()

	if version {
		about.DisplayVersion()
		return nil
	}

	var log = klogr.New().WithName("agent")
	defer klog.Flush()

	if err := validateFlags(); err != nil {
		log.Error(err, "invalid arguments")
		return err
	}

	if err := os.MkdirAll(cniConfDir, 0777); err != nil {
		log.Error(err, "failed to create cni conf dir")
		return err
	}

	manager, err := newManager(Config{
		LocalCerts:       []string{localCert},
		SyncPeriod:       AsSecond(syncPeriod),
		DebounceDuration: AsSecond(debounceDuration),

		TunnelsConfPath:  tunnelsConfPath,
		ServicesConfPath: servicesConfPath,

		MasqOutgoing: masqOutgoing,

		DummyInterfaceName: dummyInterfaceName,
		UseXfrm:            useXfrm,
		XfrmInterfaceName:  xfrmInterfaceName,
		XfrmInterfaceID:    xfrmInterfaceID,

		EnableProxy: enableProxy,

		CNI: CNI{
			Version:     cniVersion,
			ConfDir:     cniConfDir,
			NetworkName: cniNetworkName,
			BridgeName:  cniBridgeName,
		},
	})
	if err != nil {
		log.Error(err, "failed to create manager")
		return err
	}
	go manager.start()

	err = watchFiles(tunnelsConfPath, servicesConfPath, func(event fsnotify.Event) {
		log.V(5).Info("tunnels or services config may change", "file", event.Name, "event", event.Op.String())
		manager.notify()
	})

	if err != nil {
		log.Error(err, "failed to watch tunnelsconf", "file", tunnelsConfPath)
	}
	return err
}
