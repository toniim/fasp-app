//go:build windows

package recorder

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

const (
	monitorDefaultToNearest = 2
	roInitMultithreaded     = 1
)

// target is what WGC captures plus the part of each frame that is recorded.
type target struct {
	handle uintptr // HMONITOR or HWND
	window bool
	crop   d3dBox // within the captured frame
	w, h   int    // output size (even)
}

func resolveTarget(o Options) (target, error) {
	switch o.Mode {
	case ModeWindow:
		hwnd := uintptr(o.Window)
		if r, _, _ := procIsWindow.Call(hwnd); r == 0 {
			return target{}, fmt.Errorf("window 0x%X does not exist", o.Window)
		}
		// The output size is fixed by the window's size at start; the crop is
		// filled in once the capture item reports its size.
		return target{handle: hwnd, window: true}, nil

	case ModeRegion:
		r := rect{int32(o.Region.X), int32(o.Region.Y), int32(o.Region.X + o.Region.Width), int32(o.Region.Y + o.Region.Height)}
		mon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), monitorDefaultToNearest)
		mr, err := monitorRect(mon)
		if err != nil {
			return target{}, err
		}
		// Clamp to the monitor: a region spanning two monitors is cut to the
		// one holding most of it.
		l, t := max(r.Left, mr.Left), max(r.Top, mr.Top)
		rr, b := min(r.Right, mr.Right), min(r.Bottom, mr.Bottom)
		w, h := even(int(rr-l)), even(int(b-t))
		if w < 2 || h < 2 {
			return target{}, errors.New("region is not on any monitor")
		}
		x, y := uint32(l-mr.Left), uint32(t-mr.Top)
		return target{handle: mon, crop: d3dBox{x, y, 0, x + uint32(w), y + uint32(h), 1}, w: w, h: h}, nil

	default: // ModeDisplay: the monitor under the cursor, like screenshots
		var pt point
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
		mon, _, _ := procMonitorFromPoint.Call(uintptr(packed), monitorDefaultToNearest)
		mr, err := monitorRect(mon)
		if err != nil {
			return target{}, err
		}
		w, h := even(int(mr.Right-mr.Left)), even(int(mr.Bottom-mr.Top))
		return target{handle: mon, crop: d3dBox{0, 0, 0, uint32(w), uint32(h), 1}, w: w, h: h}, nil
	}
}

func monitorRect(mon uintptr) (rect, error) {
	mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if mon == 0 {
		return rect{}, errors.New("no monitor found")
	}
	if r, _, e := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return rect{}, fmt.Errorf("GetMonitorInfoW: %w", e)
	}
	return mi.RcMonitor, nil
}

type winRecording struct {
	stop     chan struct{}
	done     chan result
	ended    chan struct{} // closed when the pipeline stops, for any reason
	stopOnce sync.Once
}

type result struct {
	stats Stats
	err   error
}

// Start begins recording on a dedicated OS thread and returns once the first
// pipeline stage is up, so setup errors are reported here and not by Stop.
func Start(opts Options) (Recording, error) {
	if err := opts.normalize(); err != nil {
		return nil, err
	}
	r := &winRecording{stop: make(chan struct{}), done: make(chan result, 1), ended: make(chan struct{})}
	ready := make(chan error, 1)
	go r.run(opts, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return r, nil
}

func (r *winRecording) Done() <-chan struct{} { return r.ended }

func (r *winRecording) Stop() (Stats, error) {
	r.stopOnce.Do(func() { close(r.stop) })
	res := <-r.done
	r.done <- res // let a second Stop see the same result
	return res.stats, res.err
}

// pipeline holds every native object of one recording.
type pipeline struct {
	dev     *d3dDevice
	item    unsafe.Pointer
	cap     *wgcCapture
	enc     *encoder
	latest  unsafe.Pointer // last frame, cropped to the output size
	staging unsafe.Pointer // CPU path only
	tgt     target

	origin int64 // qpc100ns of the first frame: t=0 for video and audio
	mix    *mixer
	audio  []*wasapiCapture
}

// audioLatency is how long the mixer waits for late audio packets before it
// mixes a span; WASAPI delivers every ~10 ms, so 100 ms is generous.
const audioLatency = audioRate / 10

func (p *pipeline) stopAudio() {
	for _, c := range p.audio {
		c.close()
	}
	p.audio = nil
}

func (p *pipeline) close() {
	p.stopAudio()
	if p.cap != nil {
		p.cap.close()
	}
	release(p.staging)
	release(p.latest)
	release(p.item)
	if p.dev != nil {
		p.dev.close()
	}
}

func (r *winRecording) run(opts Options, ready chan<- error) {
	// WinRT/COM objects stay on one OS thread for their whole life.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procRoInitialize.Call(roInitMultithreaded)

	p, err := setup(opts)
	if err != nil {
		ready <- err
		return
	}
	defer p.close()
	ready <- nil

	stats, err := p.loop(opts.FPS, r.stop)
	if ferr := p.enc.finish(); err == nil {
		err = ferr
	}
	r.done <- result{stats, err}
	close(r.ended)
}

func setup(opts Options) (p *pipeline, err error) {
	p = &pipeline{}
	defer func() {
		if err != nil {
			p.close()
		}
	}()
	if p.tgt, err = resolveTarget(opts); err != nil {
		return nil, err
	}
	if p.dev, err = newD3DDevice(); err != nil {
		return nil, err
	}
	if p.item, err = newCaptureItem(p.tgt.handle, p.tgt.window); err != nil {
		return nil, fmt.Errorf("%w (Windows.Graphics.Capture needs Windows 10 1903+)", err)
	}
	if p.tgt.window {
		size, err := itemSize(p.item)
		if err != nil {
			return nil, err
		}
		p.tgt.w, p.tgt.h = even(int(size.Width)), even(int(size.Height))
		if p.tgt.w < 2 || p.tgt.h < 2 {
			return nil, errors.New("window is too small or minimized")
		}
		p.tgt.crop = d3dBox{0, 0, 0, uint32(p.tgt.w), uint32(p.tgt.h), 1}
	}
	if p.cap, err = startWGC(p.dev, p.item, opts.Cursor); err != nil {
		return nil, err
	}
	if p.latest, err = p.dev.newTexture(p.tgt.w, p.tgt.h, false); err != nil {
		return nil, err
	}

	bitrate := bitrateFor(p.tgt.w, p.tgt.h, opts.FPS, opts.Bitrate)
	audio := opts.SystemAudio || opts.Microphone
	gpu := p.dev.video && !opts.ForceCPU
	if gpu {
		p.enc, err = newEncoder(opts.Output, p.tgt.w, p.tgt.h, opts.FPS, bitrate, p.dev, true, audio)
		if err != nil { // driver without D3D11 video support: use the CPU path
			gpu = false
		}
	}
	if !gpu {
		if p.enc, err = newEncoder(opts.Output, p.tgt.w, p.tgt.h, opts.FPS, bitrate, nil, false, audio); err != nil {
			return nil, err
		}
		if p.staging, err = p.dev.newTexture(p.tgt.w, p.tgt.h, true); err != nil {
			return nil, err
		}
	}

	p.origin = qpc100ns()
	if audio {
		p.mix = &mixer{}
		add := func(name string, gain float64, loopback bool, device string) error {
			src := newAudioSource(name, gain)
			c, err := startWASAPI(src, loopback, device, p.origin)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			p.mix.sources = append(p.mix.sources, src)
			p.audio = append(p.audio, c)
			return nil
		}
		if opts.SystemAudio {
			if err = add("system audio", opts.SystemVolume, true, ""); err != nil {
				return nil, err
			}
		}
		if opts.Microphone {
			if err = add("microphone", opts.MicVolume, false, opts.MicDevice); err != nil {
				return nil, err
			}
		}
	}
	return p, nil
}

// pumpAudio mixes and encodes audio up to timeline frame upTo.
func (p *pipeline) pumpAudio(upTo int64) error {
	if p.mix == nil {
		return nil
	}
	pcm, start := p.mix.take(upTo)
	return p.enc.writeAudio(pcm, start)
}

// loop writes one frame per tick at a constant rate: the newest captured frame,
// or the previous one again when the screen did not change.
func (p *pipeline) loop(fps int, stop <-chan struct{}) (Stats, error) {
	st := Stats{Width: p.tgt.w, Height: p.tgt.h, Pipeline: "WGC + MF H.264 (CPU copy)"}
	if p.staging == nil {
		st.Pipeline = "WGC + MF H.264 (GPU zero-copy)"
	}
	interval := time.Second / time.Duration(fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	last := int64(-1)

	for {
		select {
		case <-stop:
			st.EndReason = EndStopped
			return st, p.finishStats(&st, last)
		case <-ticker.C:
		}
		if p.tgt.window {
			if ok, _, _ := procIsWindow.Call(p.tgt.handle); ok == 0 {
				st.EndReason = EndWindowClosed
				return st, p.finishStats(&st, last)
			}
		}
		n, err := p.pullFrame()
		if err != nil {
			return st, err
		}
		st.Captured += n

		// Timestamps sit on the frame grid and always move forward; slots the
		// loop was too slow for are skipped and counted as dropped.
		elapsed := qpc100ns() - p.origin
		slot := elapsed / p.enc.frameDur
		if slot <= last {
			slot = last + 1
		}
		last = slot
		if err := p.encodeLatest(slot * p.enc.frameDur); err != nil {
			return st, err
		}
		st.Frames++
		if err := p.pumpAudio(elapsed*audioRate/hundredNsPerSec - audioLatency); err != nil {
			return st, err
		}
	}
}

// finishStats fills the totals and flushes the remaining audio.
func (p *pipeline) finishStats(st *Stats, last int64) error {
	elapsed := qpc100ns() - p.origin
	st.Duration = time.Duration(elapsed * 100)
	st.Dropped = max(0, int(last+1)-st.Frames)
	if p.mix == nil {
		return nil
	}
	// Stop the devices first so every packet up to now is in, then flush the
	// tail without waiting for latency.
	p.stopAudio()
	end := elapsed * audioRate / hundredNsPerSec
	for _, src := range p.mix.sources {
		st.Audio = append(st.Audio, src.finish(end))
	}
	return p.pumpAudio(end)
}

// pullFrame copies the newest captured frame (if any) into p.latest.
func (p *pipeline) pullFrame() (int, error) {
	frame, n := p.cap.latestFrame()
	if frame == nil {
		return 0, nil
	}
	defer release(frame)
	tex, content, err := frameTexture(frame)
	if err != nil {
		return n, err
	}
	defer release(tex)

	box := p.tgt.crop
	if p.tgt.window {
		// A window can grow or shrink: record its top-left part at the size
		// it had when recording started, padding with black when smaller.
		box.Right = uint32(min(p.tgt.w, int(content.Width)))
		box.Bottom = uint32(min(p.tgt.h, int(content.Height)))
		if content != p.cap.size && content.Width > 0 && content.Height > 0 {
			defer p.cap.resize(content)
		}
	}
	tw, th := textureSize(tex)
	box.Right, box.Bottom = min(box.Right, uint32(tw)), min(box.Bottom, uint32(th))
	if box.Right > box.Left && box.Bottom > box.Top {
		p.dev.copyRegion(p.latest, tex, box)
	}
	return n, nil
}

func (p *pipeline) encodeLatest(t int64) error {
	if p.staging != nil {
		p.dev.copy(p.staging, p.latest)
		return p.dev.readStaging(p.staging, func(data unsafe.Pointer, pitch int) error {
			return p.enc.writePixels(data, pitch, t)
		})
	}
	// The encoder reads the texture asynchronously, so each sample gets its
	// own copy. Production code should pool these (IMFTrackedSample).
	tex, err := p.dev.newTexture(p.tgt.w, p.tgt.h, false)
	if err != nil {
		return err
	}
	defer release(tex)
	p.dev.copy(tex, p.latest)
	return p.enc.writeTexture(tex, t)
}
