package sync

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/go-redis/redis/v7"
	"github.com/prometheus/client_golang/prometheus"
)

// Watcher exposes methods to watch subtrees within the sync tree of this test.
type Watcher struct {
	re        *runtime.RunEnv
	client    *redis.Client
	root      string
	subs      sync.WaitGroup
	close     chan struct{}
	closeOnce sync.Once

	// metrics
	metrics struct {
		barrierTotalWait  *runtime.SummaryVec
		barrierPollsCount *runtime.CounterVec

		subtreeSubscriptionDur *runtime.SummaryVec
		subtreeEntryWait       *runtime.SummaryVec
		subtreeReceivedCount   *runtime.CounterVec
	}
}

// NewWatcher begins watching the subtree underneath this path.
//
// NOTE: Canceling the context cancels the call to this function, it does not
// affect the returned watcher.
func NewWatcher(ctx context.Context, runenv *runtime.RunEnv) (w *Watcher, err error) {
	client, err := getGlobalRedisClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("during redisClient: %w", err)
	}

	prefix := basePrefix(runenv)
	w = &Watcher{
		re:     runenv,
		client: client,
		root:   prefix,
		close:  make(chan struct{}),
	}

	w.metrics.barrierTotalWait = runenv.M().NewSummaryVec(runtime.SummaryOpts{
		Name:       "sync_watcher_barrier_total_wait_seconds",
		Help:       "sync service: watcher total barrier wait time (seconds)",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.001, 0.99: 0.001},
	}, "state")

	w.metrics.barrierPollsCount = runenv.M().NewCounterVec(runtime.CounterOpts{
		Name: "sync_watcher_barrier_polls_count",
		Help: "sync service: watcher barrier polls count",
	}, "state")

	w.metrics.subtreeReceivedCount = runenv.M().NewCounterVec(runtime.CounterOpts{
		Name: "sync_watcher_subtree_received_count",
		Help: "sync service: watcher subtree entries received count",
	}, "key")

	w.metrics.subtreeSubscriptionDur = runenv.M().NewSummaryVec(runtime.SummaryOpts{
		Name:       "sync_watcher_barrier_total_subscription_duration_seconds",
		Help:       "sync service: watcher total subscription duration (seconds)",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.001, 0.99: 0.001},
	}, "key")

	w.metrics.subtreeEntryWait = runenv.M().NewSummaryVec(runtime.SummaryOpts{
		Name:       "sync_watcher_barrier_entry_wait_time_seconds",
		Help:       "sync service: watcher entry wait time (seconds)",
		Objectives: map[float64]float64{0.5: 0.05, 0.75: 0.025, 0.9: 0.01, 0.95: 0.001, 0.99: 0.001},
	}, "key")

	return w, nil
}

// Subscribe watches a subtree and emits updates on the specified channel.
//
// The element type of the channel must match the payload type of the Subtree.
//
// We close the supplied channel when the subscription ends, in all cases. At
// that point, the caller should consume the error (or nil value) from the
// returned errCh.
//
// The user can cancel the subscription by calling the returned cancelFn or by
// canceling the passed context. The subscription will die if an internal error
// occurs.
func (w *Watcher) Subscribe(ctx context.Context, subtree *Subtree, ch interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := w.client.Context().Err(); err != nil {
		return err
	}

	chV := reflect.ValueOf(ch)
	if k := chV.Kind(); k != reflect.Chan {
		return fmt.Errorf("value is not a channel: %T", ch)
	}

	if err := subtree.AssertType(chV.Type().Elem()); err != nil {
		chV.Close()
		return err
	}

	root := w.root + ":" + subtree.GroupKey
	sub := &subscription{
		w:       w,
		subtree: subtree,
		client:  w.client.WithContext(ctx),
		key:     root,
		outCh:   chV,
	}

	// Start the subscription.
	w.subs.Add(1)
	go func() {
		defer w.subs.Done()
		sub.process()
	}()

	return nil
}

// Barrier awaits until the specified amount of items are advertising to be in
// the provided state. It returns a channel on which two things can happen:
//
//   a. if enough items appear before the context fires, a nil
//      error will be sent.
//   b. if the context fires, or another error occurs during the
//      process, an error is propagated in the channel.
//
// In both cases, the chan will only receive a single element before closure.
func (w *Watcher) Barrier(ctx context.Context, state State, required int64) <-chan error {
	log := w.re.SLogger()

	log.Debugw("setting barrier for state", "state", state, "required", required)

	resCh := make(chan error, 1)
	go func() {
		defer close(resCh)

		var (
			last   int64
			err    error
			ticker = time.NewTicker(1 * time.Second)
			k      = state.Key(w.root)
			client = w.client.WithContext(ctx)
		)

		defer ticker.Stop()

		o1 := w.metrics.barrierPollsCount.WithLabelValues((string)(state))
		o2 := w.metrics.barrierTotalWait.WithLabelValues((string)(state))
		t := prometheus.NewTimer(o2)
		defer t.ObserveDuration()

		for last < required {
			select {
			case <-ticker.C:
				o1.Inc()

				last, err = client.Get(k).Int64()
				if err != nil && err != redis.Nil {
					err = fmt.Errorf("error occured in barrier: %w", err)
					resCh <- err
					return
				}
				// loop over
				log.Debugw("insufficient instances in state; looping", "state", state, "required", required, "current", last)

			case <-ctx.Done():
				// Context fired before we got enough elements.
				err := fmt.Errorf("%s waiting on %s; not enough elements, required %d, got %d", err, state, required, last)
				resCh <- err
				return
			case <-w.close:
				resCh <- fmt.Errorf("closed")
				return
			}
		}
		if last > required {
			resCh <- fmt.Errorf("when waiting on %s; too many elements, required %d, got %d", state, required, last)
		} else {
			resCh <- nil
		}
	}()

	return resCh
}

// Close closes this watcher. After calling this method, the watcher can't be
// resused.
//
// Note: Concurrently closing the watcher while calling Subscribe may panic.
func (w *Watcher) Close() error {
	w.closeOnce.Do(func() {
		close(w.close)
		w.subs.Wait()
	})
	return nil
}
