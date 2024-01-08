package types

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

// AgentArgumentMap is used to manage arguments of agent pod
type AgentArgumentMap map[string]string

func NewAgentArgumentMap() AgentArgumentMap {
	return make(AgentArgumentMap)
}

// NewAgentArgumentMapFromEnv extract arguments of agent pod
// from ENV, each agent argument should be configured like:
//
//	AGENT_ARG_ENABLE_IPAM=true
//
// The return value is a map, each key is the ENV variable name but with
// prefix 'AGENT_ARG_' stripped and the key is also lowered.
func NewAgentArgumentMapFromEnv() AgentArgumentMap {
	const prefix = "agent-arg-"

	argMap := make(AgentArgumentMap)
	for _, line := range os.Environ() {
		parts := strings.SplitN(line, "=", 2)

		// lower variable name and replace '_' with '-'
		name := strings.ToLower(parts[0])
		name = strings.ReplaceAll(name, "_", "-")

		if !strings.HasPrefix(name, prefix) {
			continue
		}

		name = name[len(prefix):]
		value := ""
		if len(parts) > 1 {
			value = parts[1]
		}

		argMap[name] = value
	}

	return argMap
}

func (argMap AgentArgumentMap) Set(name, value string) {
	argMap[name] = value
}

func (argMap AgentArgumentMap) Get(name string) string {
	return argMap[name]
}

func (argMap AgentArgumentMap) Delete(name string) {
	delete(argMap, name)
}

func (argMap AgentArgumentMap) HasKey(name string) bool {
	_, ok := argMap[name]
	return ok
}

func (argMap AgentArgumentMap) IsProxyEnabled() bool {
	return argMap.isTrue("enable-proxy")
}

func (argMap AgentArgumentMap) IsDNSEnabled() bool {
	return argMap.isTrue("enable-dns")
}

func (argMap AgentArgumentMap) IsDNSProbeEnabled() bool {
	return argMap.isTrue("dns-probe")
}

func (argMap AgentArgumentMap) isTrue(name string) bool {
	return argMap[name] == "true"
}

// ArgumentArray translate argument map into sorted argument array
// All arguments are sorted except log level, `agent` doesn't have
// an option named log-level, instead it has a 'v' option which is used
// to configure log level. ArgumentArray will put 'v' option at the end of argument array.
func (argMap AgentArgumentMap) ArgumentArray() []string {
	const nameLogLevel = "log-level"

	nameSet := sets.NewString()
	for name := range argMap {
		nameSet.Insert(name)
	}

	args := make([]string, 0, len(argMap)+1)
	for _, name := range nameSet.List() {
		if name != nameLogLevel {
			args = append(args, fmt.Sprintf("--%s=%s", name, argMap[name]))
		}
	}

	if value, ok := argMap[nameLogLevel]; ok {
		args = append(args, fmt.Sprintf("--v=%s", value))
	}

	return args
}
