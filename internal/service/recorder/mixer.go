package recorder

import (
	"sync"
)

// Audio is mixed into one AAC track (product decision: one track plays the
// same in every player). Every source is captured as 48 kHz stereo float and
// placed on a shared timeline by the capture timestamps the OS reports, on
// the same clock as the video frames, so audio and video stay in sync.
//
// Two problems are handled here rather than in platform code:
//   - Gaps: system-audio loopback delivers nothing while the machine is
//     silent; the timeline is filled with silence.
//   - Clock drift: a microphone runs on its own crystal, a little faster or
//     slower than the system clock. When a source's position drifts more
//     than driftTolerance away from its timestamps, it is snapped back by
//     inserting silence or dropping samples.
//     (Production: adaptive resampling instead of snapping.)

const (
	audioRate      = 48000
	audioChannels  = 2
	driftTolerance = 960 // frames (20 ms) of timestamp jitter tolerated
	// A jump longer than this is the source going quiet (loopback sends no
	// packets while nothing plays), not clock drift.
	silenceGap = audioRate / 10
	// audioLatency is how long the mixer waits for late audio packets before
	// it mixes a span; devices deliver every ~10-20 ms, so 100 ms is generous.
	audioLatency = audioRate / 10
)

// AudioStats describe one audio source of a finished recording.
type AudioStats struct {
	Name string
	// Received is the number of frames the device delivered.
	Received int64
	// GapFrames were filled with silence because the source delivered
	// nothing (silence on loopback, or the source running slow).
	GapFrames int64
	// Continuous is false when the source paused (silent loopback); DriftPPM
	// is then not measured.
	Continuous bool
	// DroppedFrames were discarded because the source ran ahead of its
	// timestamps or arrived too late to be mixed.
	DroppedFrames int64
	// Snaps counts drift corrections after the source had started.
	Snaps int
	// DriftPPM estimates how much faster (+) or slower (-) the device clock
	// runs than the system clock. Only meaningful for continuous sources.
	DriftPPM float64
	// Reopens counts device restarts (default device changed, unplugged).
	Reopens int
	Err     string
}

type audioSource struct {
	name string
	gain float32

	mu      sync.Mutex
	buf     []float32 // interleaved stereo starting at timeline frame `base`
	base    int64
	started bool
	first   int64 // timeline frame of the first packet
	drift   int64 // net frames corrected, signed: + source slow, - source fast
	lastD   int64 // offset of the newest packet from where it was expected
	paused  bool  // the source stopped delivering for a while
	stats   AudioStats
}

func newAudioSource(name string, gain float64) *audioSource {
	return &audioSource{name: name, gain: float32(gain), stats: AudioStats{Name: name}}
}

// push adds a packet whose first frame was captured at timeline frame pos.
// data is interleaved stereo; nil means a packet of n silent frames.
func (s *audioSource) push(pos int64, data []float32, n int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.Received += n
	if pos+n <= 0 { // captured before the recording started
		return
	}
	if pos < 0 {
		data, n = trimFront(data, -pos), n+pos
		pos = 0
	}

	w := s.base + int64(len(s.buf)/audioChannels) // where the next frame would go
	d := pos - w
	s.lastD = d
	if !s.started {
		s.started, s.first = true, pos
		s.lastD = 0
		if d > 0 { // lead-in silence before the source's first packet
			s.buf = append(s.buf, make([]float32, d*audioChannels)...)
		} else if d < 0 { // the mixer already moved past the start of this packet
			drop := min(-d, n)
			data, n = trimFront(data, drop), n-drop
			if n == 0 {
				return
			}
		}
	} else if d > driftTolerance {
		s.buf = append(s.buf, make([]float32, d*audioChannels)...)
		s.stats.GapFrames += d
		s.lastD = 0
		if d > silenceGap {
			s.paused = true
		} else {
			s.stats.Snaps++
			s.drift += d
		}
	} else if d < -driftTolerance {
		drop := min(-d, n)
		data, n = trimFront(data, drop), n-drop
		s.stats.DroppedFrames += drop
		s.stats.Snaps++
		s.drift -= drop
		s.lastD = 0
		if n == 0 {
			return
		}
	}
	if data == nil {
		s.buf = append(s.buf, make([]float32, n*audioChannels)...)
	} else {
		s.buf = append(s.buf, data[:n*audioChannels]...)
	}
}

func trimFront(data []float32, frames int64) []float32 {
	if data == nil {
		return nil
	}
	return data[frames*audioChannels:]
}

func (s *audioSource) finish(end int64) AudioStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats
	st.Continuous = s.started && !s.paused
	if span := end - s.first; st.Continuous && span > audioRate {
		// Net correction plus the residual offset, over the time it built up.
		st.DriftPPM = -float64(s.drift+s.lastD) / float64(span) * 1e6
	}
	return st
}

// mixer turns the sources into one stereo 16-bit PCM stream.
type mixer struct {
	sources []*audioSource
	emitted int64 // timeline frames already handed to the encoder
}

// take mixes timeline frames [emitted, upTo). A source that has not delivered
// them by now contributes silence, and whatever it delivers later for that
// span is dropped by push.
func (m *mixer) take(upTo int64) (pcm []int16, start int64) {
	n := upTo - m.emitted
	if n <= 0 {
		return nil, m.emitted
	}
	mix := make([]float32, n*audioChannels)
	for _, s := range m.sources {
		s.mu.Lock()
		have := min(int64(len(s.buf)/audioChannels), n)
		for i, v := range s.buf[:have*audioChannels] {
			mix[i] += v * s.gain
		}
		s.buf = append(s.buf[:0], s.buf[have*audioChannels:]...)
		s.base = upTo
		s.mu.Unlock()
	}
	pcm = make([]int16, len(mix))
	for i, v := range mix {
		pcm[i] = floatToPCM16(v)
	}
	start = m.emitted
	m.emitted = upTo
	return pcm, start
}

func floatToPCM16(v float32) int16 {
	switch {
	case v >= 1:
		return 32767
	case v <= -1:
		return -32768
	}
	return int16(v * 32767)
}
