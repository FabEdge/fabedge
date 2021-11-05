package routines

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func Periodic(interval time.Duration, fn func(ctx context.Context)) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		tick := time.NewTicker(interval)

		fn(ctx)
		for {
			select {
			case <-tick.C:
				fn(ctx)
			case <-ctx.Done():
				return nil
			}
		}
	})
}
