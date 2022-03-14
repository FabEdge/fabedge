package routines

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/apiserver"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
)

var _ = Describe("LoadEndpointsAndCommunities", func() {
	It("can load endpoints and communities", func() {
		e1 := apis.Endpoint{
			Name:            "cluster1.connector",
			PublicAddresses: []string{"cluster1"},
			Subnets:         []string{"2.2.2.0/24"},
			NodeSubnets:     []string{"10.10.0.1/32"},
		}
		e2 := apis.Endpoint{
			Name:            "cluster2.connector",
			PublicAddresses: []string{"cluster2.connector"},
			Subnets:         []string{"192.168.1.0/24"},
			NodeSubnets:     []string{"192.168.1.1/32"},
		}
		e3 := apis.Endpoint{
			Name:            "cluster2.edge",
			PublicAddresses: []string{"cluster2"},
			Subnets:         []string{"192.168.2.0/24"},
			NodeSubnets:     []string{"192.168.1.2/32"},
		}

		var ec = apiserver.EndpointsAndCommunity{
			Communities: map[string][]string{
				"connectors": {e1.Name, e2.Name},
			},
			Endpoints: []apis.Endpoint{e1, e2},
		}

		store := storepkg.NewStore()
		var lock sync.RWMutex
		getEndpointsAndCommunities := func() (apiserver.EndpointsAndCommunity, error) {
			lock.RLock()
			defer lock.RUnlock()

			return ec, nil
		}

		loader := LoadEndpointsAndCommunities(10*time.Millisecond, store, getEndpointsAndCommunities)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go loader.Start(ctx)

		time.Sleep(50 * time.Millisecond)

		community, _ := store.GetCommunity("connectors")
		Expect(community.Members.List()).Should(ConsistOf(e1.Name, e2.Name))

		endpoint, _ := store.GetEndpoint(e1.Name)
		Expect(endpoint).Should(Equal(e1))

		endpoint, _ = store.GetEndpoint(e2.Name)
		Expect(endpoint).Should(Equal(e2))

		By("change endpoints and communities")
		lock.Lock()
		ec.Communities = map[string][]string{
			"mixed": {e1.Name, e3.Name},
			"void":  {"edge1", "edge2"},
		}
		ec.Endpoints = []apis.Endpoint{e1, e3}
		lock.Unlock()

		time.Sleep(50 * time.Millisecond)

		community, ok := store.GetCommunity("connectors")
		Expect(ok).Should(BeFalse())

		community, ok = store.GetCommunity("void")
		Expect(community.Members.List()).Should(ConsistOf("edge1", "edge2"))

		community, ok = store.GetCommunity("mixed")
		Expect(ok).Should(BeTrue())
		Expect(community.Members.List()).Should(ConsistOf(e1.Name, e3.Name))

		endpoint, _ = store.GetEndpoint(e1.Name)
		Expect(endpoint).Should(Equal(e1))

		endpoint, _ = store.GetEndpoint(e3.Name)
		Expect(endpoint).Should(Equal(e3))

		_, ok = store.GetEndpoint(e2.Name)
		Expect(ok).Should(BeFalse())
	})
})
