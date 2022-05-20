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

package connector

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

func eventOpIs(ent fsnotify.Event, Op fsnotify.Op) bool {
	return ent.Op&Op == Op
}

func (m *Manager) onConfigFileChange(fileToWatch string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.log.Error(err, "failed to initialize fsnotify")
		return
	}

	defer func() {
		if err = watcher.Close(); err != nil {
			m.log.Error(err, "failed to close fsnotify watcher")
		}
	}()

	if err = watcher.Add(fileToWatch); err != nil {
		klog.Errorf("failed to monitor %s. Error: %s", fileToWatch, err)
		return
	}

	for {
		select {
		case event, _ := <-watcher.Events:
			switch {
			case eventOpIs(event, fsnotify.Remove):
				m.log.Info("file removed, add it back", "event", event)
				if err = watcher.Add(fileToWatch); err != nil {
					m.log.Error(err, "failed to watch file", "file", fileToWatch)
				}
			default:
				m.log.Info("file changed, start to sync", "event", event)
				m.notify()
			}

		case err, _ = <-watcher.Errors:
			m.log.Error(err, "fsnotify has an error")
			// not encounter it so far, hope it can be recovered after some time
			time.Sleep(5 * time.Minute)
			if err = watcher.Add(fileToWatch); err != nil {
				m.log.Error(err, "failed to monitor file", "file", fileToWatch)
				return
			}
		}
	}
}
