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

package e2e

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/fabedge/fabedge/test/e2e/framework"
)

func expectCurlResultContains(cluster *Cluster, pod corev1.Pod, url string, substr string) {
	timeout := fmt.Sprint(framework.TestContext.CurlTimeout)
	err := wait.Poll(time.Second, time.Duration(framework.TestContext.CurlTimeout)*time.Second, func() (bool, error) {
		stdout, _, _ := cluster.execute(pod, []string{"curl", "-sS", "-m", timeout, url})
		return strings.Contains(stdout, substr), nil
	})

	Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("response of curl %s should contains %s", url, substr))
}

func getName(prefix string) string {
	time.Sleep(time.Millisecond)
	return fmt.Sprintf("%s-%d", prefix, rand.Int31n(1000))
}
