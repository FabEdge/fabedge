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

package agent

import (
	"context"
	"math"
	"time"

	"github.com/avast/retry-go"
)

// retryForever retry fn until it success
func retryForever(ctx context.Context, retryableFunc retry.RetryableFunc, onRetryFunc retry.OnRetryFunc) {
	_ = retry.Do(
		retryableFunc,
		retry.Context(ctx),
		retry.Attempts(math.MaxUint64),
		retry.Delay(5*time.Second),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
		retry.OnRetry(onRetryFunc),
	)
}
