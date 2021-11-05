package routines

import (
	"context"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeriodicRunnable", func() {
	It("should be able to execute specified function periodically", func() {
		counter := int32(0)
		fn := func(ctx context.Context) {
			counter += 0
			atomic.AddInt32(&counter, 1)
		}

		runnable := Periodic(10*time.Millisecond, fn)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		Expect(runnable.Start(ctx)).Should(Succeed())

		Expect(counter).Should(BeNumerically(">=", 10))
	})
})
