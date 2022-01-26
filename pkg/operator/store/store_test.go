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

package store_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("Store", func() {
	var store storepkg.Interface

	BeforeEach(func() {
		store = storepkg.NewStore()
	})

	It("support endpoint CRUD operations", func() {
		e1 := apis.Endpoint{
			ID:              "edge1",
			Name:            "edge1",
			PublicAddresses: []string{"10.40.20.181"},
			Subnets:         []string{"2.2.0.0/26"},
		}

		e2 := apis.Endpoint{
			ID:              "edge2",
			Name:            "edge2",
			PublicAddresses: []string{"10.40.20.182"},
			Subnets:         []string{"2.2.0.64/26"},
		}

		store.SaveEndpoint(e1)
		store.SaveEndpointAsLocal(e2)

		e, ok := store.GetEndpoint(e1.Name)
		Expect(ok).To(BeTrue())
		Expect(e).To(Equal(e1))

		nameSet := store.GetAllEndpointNames()
		Expect(nameSet.Has(e1.Name)).To(BeTrue())
		Expect(nameSet.Has(e2.Name)).To(BeTrue())

		nameSet = store.GetLocalEndpointNames()
		Expect(nameSet.Has(e1.Name)).To(BeFalse())
		Expect(nameSet.Has(e2.Name)).To(BeTrue())

		endpoints2 := store.GetEndpoints(e1.Name, e2.Name)
		Expect(endpoints2).To(ContainElement(e1))
		Expect(endpoints2).To(ContainElement(e2))

		store.DeleteEndpoint(e2.Name)
		e, ok = store.GetEndpoint(e2.Name)
		Expect(ok).To(BeFalse())
		Expect(e).NotTo(Equal(e1))

		nameSet = store.GetLocalEndpointNames()
		Expect(nameSet.Has(e2.Name)).To(BeFalse())
	})

	It("can support community CRUD operations", func() {
		c1 := types.Community{
			Name:    "nginx",
			Members: sets.NewString("edge1", "edge2"),
		}
		c2 := types.Community{
			Name:    "apache",
			Members: sets.NewString("edge1", "edge2", "edge3"),
		}

		store.SaveCommunity(c1)
		store.SaveCommunity(c2)

		c, ok := store.GetCommunity(c1.Name)
		Expect(ok).To(BeTrue())
		Expect(c).To(Equal(c1))

		communities := store.GetCommunitiesByEndpoint("edge1")
		Expect(communities).To(ContainElement(c1))
		Expect(communities).To(ContainElement(c2))

		communities = store.GetCommunitiesByEndpoint("edge3")
		Expect(communities).To(ContainElement(c2))
		Expect(communities).NotTo(ContainElement(c1))

		c2 = types.Community{
			Name:    "apache",
			Members: sets.NewString("edge1", "edge3"),
		}
		store.SaveCommunity(c2)
		communities = store.GetCommunitiesByEndpoint("edge2")
		Expect(communities).To(ContainElement(c1))
		Expect(communities).NotTo(ContainElement(c2))

		store.DeleteCommunity(c1.Name)
		communities = store.GetCommunitiesByEndpoint("edge1")
		Expect(communities).NotTo(ContainElement(c1))
		Expect(communities).To(ContainElement(c2))

		c, ok = store.GetCommunity(c1.Name)
		Expect(ok).To(BeFalse())
		Expect(c).NotTo(Equal(c1))
	})
})
