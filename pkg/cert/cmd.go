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

package cert

import (
	"flag"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
)

func Execute() error {
	klog.InitFlags(nil)
	_ = flag.Set("v", "3")
	flag.Parse()

	if version {
		about.DisplayVersion()
		return nil
	}

	var log = klogr.New().WithName("cert")
	defer klog.Flush()

	if err := validateFlags(); err != nil {
		log.Error(err, "invalid arguments")
		return err
	}

	manager, err := initManager()
	if err != nil {
		log.Error(err, "failed to init cert manager")
		return err
	}

	if signConnectorCert {
		if err := manager.SignConnectorCert(); err != nil {
			log.Error(err, "failed to sign certs for connector tunnel")
			return err
		}
	}

	if signEdgeCert {
		if err := manager.SignEdgeCertAndPersistence(); err != nil {
			log.Error(err, "failed to sign certs for connector tunnel", "commonName", manager.commonName)
			return err
		}
	}

	return nil
}
