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

package about

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

var (
	version     = "0.0.0"                // semantic version X.Y.Z
	gitCommit   = "00000000"             // sha1 from git
	buildTime   = "1970-01-01T00:00:00Z" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	showVersion *bool
)

func AddFlags(fs *pflag.FlagSet) {
	showVersion = fs.Bool("show-version", false, "Display version info")
}

func DisplayAndExitIfRequested() {
	if *showVersion {
		DisplayVersion()
		os.Exit(0)
	}
}

func DisplayVersion() {
	fmt.Printf("Version: %s\nBuildTime: %s\nGitCommit: %s\n", version, buildTime, gitCommit)
}
