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
	"os"

	"github.com/fsnotify/fsnotify"
	flag "github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
)

func Execute() error {
	defer klog.Flush()

	fs := flag.CommandLine
	cfg := &Config{}

	about.AddFlags(fs)
	logutil.AddFlags(fs)
	cfg.AddFlags(fs)

	flag.Parse()

	about.DisplayAndExitIfRequested()

	log := klogr.New().WithName("manager")
	if err := cfg.Validate(); err != nil {
		log.Error(err, "validation failed")
		return err
	}

	if err := os.MkdirAll(cfg.CNI.ConfDir, 0777); err != nil {
		log.Error(err, "failed to create cni conf dir")
		return err
	}

	manager, err := cfg.Manager()
	if err != nil {
		log.Error(err, "failed to create manager")
		return err
	}

	go manager.start()

	err = watchFiles(cfg.TunnelsConfPath, cfg.ServicesConfPath, func(event fsnotify.Event) {
		log.V(5).Info("tunnels or services config may change", "file", event.Name, "event", event.Op.String())
		manager.notify()
	})

	if err != nil {
		log.Error(err, "failed to watch tunnelsconf", "file", cfg.TunnelsConfPath)
	}
	return err
}
