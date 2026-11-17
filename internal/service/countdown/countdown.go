// Package countdown runs the timer behind the delayed capture items: it counts
// whole seconds down, reports each one, and then fires exactly once, unless it
// is cancelled first.
package countdown

import (
	"sync"
	"time"
)

// Timer runs one countdown at a time.
type Timer struct {
	// Interval is the length of one tick. It is one second unless a test
	// shortens it.
	Interval time.Duration

	mu        sync.Mutex
	cancel    chan struct{} // set while a countdown runs
	cancelled bool          // Cancel has closed cancel
}

// New returns a timer that ticks once a second.
func New() *Timer {
	return &Timer{Interval: time.Second}
}

// Running reports whether a countdown is in progress and not cancelled.
func (t *Timer) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cancel != nil && !t.cancelled
}

// Start begins counting down from seconds. It calls tick(n) for n = seconds
// down to 1, one per interval, and then done(true). If Cancel is called before
// the end, it calls done(false) instead. done is always called exactly once,
// after the last tick, and the timer is idle again by the time it runs, so
// done may start a new countdown. Start returns false, and does nothing, when
// a countdown is already running or seconds is below 1. tick and done run on
// the timer's goroutine.
func (t *Timer) Start(seconds int, tick func(remaining int), done func(fired bool)) bool {
	if seconds < 1 {
		return false
	}
	t.mu.Lock()
	if t.cancel != nil {
		t.mu.Unlock()
		return false
	}
	cancel := make(chan struct{})
	t.cancel, t.cancelled = cancel, false
	t.mu.Unlock()

	go func() {
		fired := t.run(seconds, cancel, tick)
		t.mu.Lock()
		t.cancel = nil
		t.mu.Unlock()
		done(fired)
	}()
	return true
}

func (t *Timer) run(seconds int, cancel <-chan struct{}, tick func(int)) bool {
	for remaining := seconds; remaining >= 1; remaining-- {
		tick(remaining)
		select {
		case <-cancel:
			return false
		case <-time.After(t.Interval):
		}
	}
	// A cancel that raced with the last tick still wins.
	select {
	case <-cancel:
		return false
	default:
		return true
	}
}

// Cancel stops the running countdown. It returns false when none was running.
func (t *Timer) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	// The channel stays set until the goroutine has finished, so a new Start
	// cannot overlap the old countdown's done call.
	if t.cancel == nil || t.cancelled {
		return false
	}
	t.cancelled = true
	close(t.cancel)
	return true
}
