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

package agent

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/avast/retry-go"

	sysctlutil "github.com/fabedge/fabedge/third_party/sysctl"
)

var sysctl = sysctlutil.New()

// retryForever retry fn until it succeed
func retryForever(ctx context.Context, retryableFunc retry.RetryableFunc, onRetryFunc retry.OnRetryFunc) {
	_ = retry.Do(
		retryableFunc,
		retry.Context(ctx),
		retry.Attempts(math.MaxUint32),
		retry.Delay(5*time.Second),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(onRetryFunc),
	)
}

// ensureSysctl sets a kernel sysctl to a given numeric value.
func ensureSysctl(name string, newVal int) error {
	if oldVal, _ := sysctl.GetSysctl(name); oldVal != newVal {
		if err := sysctl.SetSysctl(name, newVal); err != nil {
			return fmt.Errorf("can't set sysctl %s to %d: %v", name, newVal, err)
		}
	}
	return nil
}
