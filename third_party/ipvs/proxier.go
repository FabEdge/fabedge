/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipvs

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/klog/v2"
	utilexec "k8s.io/utils/exec"
)

// DefaultScheduler is the default ipvs scheduler algorithm - round robin.
const DefaultScheduler = "rr"

// KernelHandler can handle the current installed kernel modules.
type KernelHandler interface {
	GetModules() ([]string, error)
	GetKernelVersion() (string, error)
}

// LinuxKernelHandler implements KernelHandler interface.
type LinuxKernelHandler struct {
	executor utilexec.Interface
}

// NewLinuxKernelHandler initializes LinuxKernelHandler with exec.
func NewLinuxKernelHandler() *LinuxKernelHandler {
	return &LinuxKernelHandler{
		executor: utilexec.New(),
	}
}

// GetModules returns all installed kernel modules.
func (handle *LinuxKernelHandler) GetModules() ([]string, error) {
	// Check whether IPVS required kernel modules are built-in
	kernelVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return nil, err
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	ipvsModules := GetRequiredIPVSModules(kernelVersion)

	var bmods, lmods []string

	// Find out loaded kernel modules. If this is a full static kernel it will try to verify if the module is compiled using /boot/config-KERNELVERSION
	modulesFile, err := os.Open("/proc/modules")
	if err == os.ErrNotExist {
		klog.Warningf("Failed to read file /proc/modules with error %v. Assuming this is a kernel without loadable modules support enabled", err)
		kernelConfigFile := fmt.Sprintf("/boot/config-%s", kernelVersionStr)
		kConfig, err := ioutil.ReadFile(kernelConfigFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read Kernel Config file %s with error %v", kernelConfigFile, err)
		}
		for _, module := range ipvsModules {
			if match, _ := regexp.Match("CONFIG_"+strings.ToUpper(module)+"=y", kConfig); match {
				bmods = append(bmods, module)
			}
		}
		return bmods, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read file /proc/modules with error %v", err)
	}

	mods, err := getFirstColumn(modulesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to find loaded kernel modules: %v", err)
	}

	builtinModsFilePath := fmt.Sprintf("/lib/modules/%s/modules.builtin", kernelVersionStr)
	b, err := ioutil.ReadFile(builtinModsFilePath)
	if err != nil {
		klog.Warningf("Failed to read file %s with error %v. You can ignore this message when kube-proxy is running inside container without mounting /lib/modules", builtinModsFilePath, err)
	}

	for _, module := range ipvsModules {
		if match, _ := regexp.Match(module+".ko", b); match {
			bmods = append(bmods, module)
		} else {
			// Try to load the required IPVS kernel modules if not built in
			err := handle.executor.Command("modprobe", "--", module).Run()
			if err != nil {
				klog.Warningf("Failed to load kernel module %v with modprobe. "+
					"You can ignore this message when kube-proxy is running inside container without mounting /lib/modules", module)
			} else {
				lmods = append(lmods, module)
			}
		}
	}

	mods = append(mods, bmods...)
	mods = append(mods, lmods...)
	return mods, nil
}

// getFirstColumn reads all the content from r into memory and return a
// slice which consists of the first word from each line.
func getFirstColumn(r io.Reader) ([]string, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(b), "\n")
	words := make([]string, 0, len(lines))
	for i := range lines {
		fields := strings.Fields(lines[i])
		if len(fields) > 0 {
			words = append(words, fields[0])
		}
	}
	return words, nil
}

// GetKernelVersion returns currently running kernel version.
func (handle *LinuxKernelHandler) GetKernelVersion() (string, error) {
	kernelVersionFile := "/proc/sys/kernel/osrelease"
	fileContent, err := ioutil.ReadFile(kernelVersionFile)
	if err != nil {
		return "", fmt.Errorf("error reading osrelease file %q: %v", kernelVersionFile, err)
	}

	return strings.TrimSpace(string(fileContent)), nil
}

// CanUseIPVSProxier returns true if we can use the ipvs Proxier.
// This is determined by checking if all the required kernel modules can be loaded. It may
// return an error if it fails to get the kernel modules information without error, in which
// case it will also return false.
func CanUseIPVSProxier(handle KernelHandler) (bool, error) {
	mods, err := handle.GetModules()
	if err != nil {
		return false, fmt.Errorf("error getting installed ipvs required kernel modules: %v", err)
	}
	loadModules := sets.NewString()
	loadModules.Insert(mods...)

	kernelVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return false, fmt.Errorf("error determining kernel version to find required kernel modules for ipvs support: %v", err)
	}
	kernelVersion, err := version.ParseGeneric(kernelVersionStr)
	if err != nil {
		return false, fmt.Errorf("error parsing kernel version %q: %v", kernelVersionStr, err)
	}
	mods = GetRequiredIPVSModules(kernelVersion)
	wantModules := sets.NewString()
	wantModules.Insert(mods...)

	modules := wantModules.Difference(loadModules).UnsortedList()
	var missingMods []string
	ConntrackiMissingCounter := 0
	for _, mod := range modules {
		if strings.Contains(mod, "nf_conntrack") {
			ConntrackiMissingCounter++
		} else {
			missingMods = append(missingMods, mod)
		}
	}
	if ConntrackiMissingCounter == 2 {
		missingMods = append(missingMods, "nf_conntrack_ipv4(or nf_conntrack for Linux kernel 4.19 and later)")
	}

	if len(missingMods) != 0 {
		return false, fmt.Errorf("IPVS proxier will not be used because the following required kernel modules are not loaded: %v", missingMods)
	}

	return true, nil
}

func SupportXfrmInterface(handle KernelHandler) (bool, error) {
	currentVersionStr, err := handle.GetKernelVersion()
	if err != nil {
		return false, err
	}

	currentVersion, err := version.ParseGeneric(currentVersionStr)
	if err != nil {
		return false, err
	}

	// xfrm interfaces have been supported since kernel 4.19
	// see http://patchwork.ozlabs.org/project/netdev/patch/20190101124001.28e58469@aquamarine/
	xfrmMinSupportedKernelVersion := "4.19"
	expectVersion, err := version.ParseGeneric(xfrmMinSupportedKernelVersion)
	if err != nil {
		return false, err
	}

	return expectVersion.LessThan(currentVersion), nil
}
