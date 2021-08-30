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

package routing

import (
	"github.com/fabedge/fabedge/pkg/tunnel"
)

type FlannelRouter struct {
}

func NewFlannelRouter() *FlannelRouter {
	return &FlannelRouter{}
}

func (c *FlannelRouter) SyncRoutes(active bool, connections []tunnel.ConnConfig) error {
	switch active {
	case true:
		if err := delRoutesNotInConnections(connections, TableStrongswan); err != nil {
			return err
		}
		if err := addAllEdgeRoutes(connections, TableStrongswan); err != nil {
			return err
		}
	case false:
		if err := c.CleanRoutes(connections); err != nil {
			return err
		}
	}

	return nil
}

func (c *FlannelRouter) CleanRoutes(conns []tunnel.ConnConfig) error {
	// delete routes in table 220
	return delAllEdgeRoutes(conns)
}
