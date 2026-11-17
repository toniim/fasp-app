package main

import (
	"strconv"
	"time"

	"github.com/heytonyne/fasp/internal/service/countdown"
)

// cancelKey cancels a running countdown from anywhere on screen. It is a global
// hotkey held only while the countdown runs.
const cancelKey = "Escape"

// trayRepaintDelay is how long the countdown stays gone from the menu bar
// before the screen is captured. The seconds in the menu bar would otherwise
// be in the picture; this gives the menu bar time to redraw without them.
const trayRepaintDelay = 350 * time.Millisecond

// delayedCapture is the state of the "Capture in N s" tray items.
type delayedCapture struct {
	timer *countdown.Timer
}

// startDelayed counts down from seconds, showing the seconds left on the tray,
// and then emits event (the same event the capture hotkey emits, so the
// capture opens in the editor as usual). Choosing a delayed item while a
// countdown runs cancels it, as does Esc or the "Cancel Countdown" item.
func (a *App) startDelayed(event string, seconds int) {
	if a.delayed.timer == nil {
		a.delayed.timer = countdown.New()
	}
	t := a.delayed.timer
	if t.Running() {
		t.Cancel()
		return
	}
	if a.IsRecording() {
		println("[capture] delayed capture ignored: a recording is running")
		return
	}

	a.holdCancelKey(t.Cancel)
	started := t.Start(seconds,
		func(remaining int) {
			if a.trayManager != nil {
				a.trayManager.SetCountdown(remaining)
			}
		},
		func(fired bool) {
			a.releaseCancelKey()
			if a.trayManager != nil {
				a.trayManager.SetCountdown(0)
			}
			if !fired {
				println("[capture] delayed capture cancelled")
				return
			}
			time.Sleep(trayRepaintDelay)
			println("[capture] delayed capture fired:", event)
			a.emit(event)
		})
	if !started {
		a.releaseCancelKey()
	}
}

// cancelCountdown stops a running countdown; it reports whether one ran.
func (a *App) cancelCountdown() bool {
	return a.delayed.timer != nil && a.delayed.timer.Cancel()
}

// delayFromArg reads the seconds a tray delay item carries.
func delayFromArg(arg string) (int, bool) {
	n, err := strconv.Atoi(arg)
	return n, err == nil && n >= 1
}

func (a *App) holdCancelKey(cancel func() bool) {
	if a.hotkeyService == nil {
		return
	}
	if err := a.hotkeyService.Register(cancelKey, func() { cancel() }); err != nil {
		println("[capture] Esc cannot cancel the countdown:", err.Error())
		return
	}
	// The listener only runs once started; "already running" is not a problem.
	_ = a.hotkeyService.Start()
}

func (a *App) releaseCancelKey() {
	if a.hotkeyService == nil {
		return
	}
	_ = a.hotkeyService.Unregister(cancelKey)
}
