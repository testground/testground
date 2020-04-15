package sync

import (
	"time"

	"github.com/go-redis/redis/v7"
)

// GCLastAccessThreshold specifies the minimum amount of time that should've
// elapsed since a Redis key was last accessed to be pruned by garbage
// collection.
var GCLastAccessThreshold = 30 * time.Minute

// GCFrequency is the frequency at which periodic GC runs, if enabled.
var GCFrequency = 30 * time.Minute

// EnableBackgroundGC enables a background process to perform periodic GC.
// It ticks once when called, then with GCFrequency frequency.
//
// An optional notifyCh can be passed in to be notified everytime GC runs, with
// the result of each run. This is mostly used for testing.
func (c *Client) EnableBackgroundGC(notifyCh chan error) {
	go func() {
		for {
			err := c.RunGC()

			select {
			case notifyCh <- err:
			default:
			}

			select {
			case <-time.After(GCFrequency):
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

// RunGC runs a round of GC. GC consists of paging through the Redis database
// with SCAN, fetching the last access time of all keys via a pipelined OBJECT
// IDLETIME, and deleting the keys that have been idle for greater or equal to
// GCLastAccessThreshold.
func (c *Client) RunGC() error {
	var (
		del    []string // delete set, recycled.
		cursor uint64   // Redis cursor ID, reset on every iteration.
		purged int64
	)

	c.log.Infow("sync gc: running", "expiry_threshold", GCLastAccessThreshold)
	defer func() { c.log.Infow("sync gc: finished", "purged", purged) }()

	for ok := true; ok; ok = cursor != 0 {
		del = del[:0]

		keys, crsor, err := c.rclient.Scan(cursor, "", 50).Result()
		if err != nil {
			c.log.Warnw("sync gc: failed to scan keys", "error", err)
			return err
		}

		cursor = crsor

		// Execute a Redis pipeline to fetch idle times for keys.
		p := c.rclient.Pipeline()
		idletimes := make([]*redis.DurationCmd, 0, len(keys))
		for _, k := range keys {
			idletimes = append(idletimes, p.ObjectIdleTime(k))
		}
		if _, err = p.Exec(); err != nil {
			c.log.Warnw("sync gc: failed to obtain object idle times", "error", err)
			return err
		}

		// Process idle times and populate the deletion set.
		for i, it := range idletimes {
			t, err := it.Result()
			if err != nil {
				c.log.Warnw("sync gc: failed to obtain object idle time for key; skipping", "key", keys[i], "error", err)
				return err
			}
			if t >= GCLastAccessThreshold {
				del = append(del, keys[i])
			}
		}

		if len(del) == 0 {
			// nothing to delete in this iteration.
			continue
		}

		// Delete the keys that have been inactive for too long.
		delcnt, err := c.rclient.Del(del...).Result()
		if err != nil {
			c.log.Warnw("sync gc: failed to delete keys", "error", err)
			return err
		}

		// Check that the expected amount of keys were deleted.
		if l := len(del); int64(l) != delcnt {
			c.log.Warnw("sync gc: less keys deleted than expected", "expected", l, "actual", delcnt)
		}

		purged += delcnt
	}
	return nil
}
