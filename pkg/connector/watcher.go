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
	"github.com/bep/debounce"
	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
	"time"
)

func eventOpIs(ent fsnotify.Event, Op fsnotify.Op) bool {
	return ent.Op&Op == Op
}

func (m *Manager) onConfigFileChange(fileToWatch string, callback func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Errorf("failed to initialize fsnotify: %s", err)
		return
	}
	defer func() {
		if err = watcher.Close(); err != nil {
			klog.Errorf("failed to close fsnotify watcher: %s", err)
		}
	}()

	if err = watcher.Add(fileToWatch); err != nil {
		klog.Errorf("failed to monitor %s. Error: %s", fileToWatch, err)
		return
	}

	// use debounce to avoid too much fsnotify events
	debounced := debounce.New(m.config.debounceDuration)

	for {
		select {
		case event, _ := <-watcher.Events:
			klog.Infof("network configuration may changed. event: %s", event)
			switch {
			case eventOpIs(event, fsnotify.Create):
				fallthrough
			case eventOpIs(event, fsnotify.Write):
				fallthrough
			case eventOpIs(event, fsnotify.Rename):
				debounced(callback)
			default:
				// fsnotify monitors the inode, but vi change the inode.
				// try to add it back, after removed/renamed
				time.Sleep(1 * time.Second) // wait fs to ready, maybe not needed
				if err = watcher.Add(fileToWatch); err != nil {
					klog.Errorf("failed to monitor %s. Error: %s", fileToWatch, err)
					return
				}
			}
		case err, _ = <-watcher.Errors:
			klog.Errorf("fsnotify has an error: %s", err)
			// not encounter it so far, hope it can be recovered after some time
			time.Sleep(5 * time.Minute)
			if err = watcher.Add(fileToWatch); err != nil {
				klog.Errorf("failed to monitor %s. Error: %s", fileToWatch, err)
				return
			}
		}
	}
}
