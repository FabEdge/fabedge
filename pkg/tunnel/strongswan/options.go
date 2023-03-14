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

package strongswan

type Options []option
type option func(manager *StrongSwanManager)

func SocketFile(path string) option {
	return func(m *StrongSwanManager) {
		m.socketPath = path
	}
}

func CertsDir(path string) option {
	return func(m *StrongSwanManager) {
		m.certsPath = path
	}
}

func StartAction(startAction string) option {
	return func(m *StrongSwanManager) {
		m.startAction = startAction
	}
}

func DpdAction(action string) option {
	return func(m *StrongSwanManager) {
		m.dpdAction = action
	}
}

func DpdDelay(delay string) option {
	return func(m *StrongSwanManager) {
		m.dpdDelay = delay
	}
}

func InterfaceID(id *uint) option {
	return func(m *StrongSwanManager) {
		m.interfaceID = id
	}
}

// InitTimeout set timeout for SA/child-SA initiation. 0 means blocking initiation
// unit: second.
func InitTimeout(timeout uint) option {
	return func(m *StrongSwanManager) {
		m.initTimeout = timeout * 1000
	}
}
