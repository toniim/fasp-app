package recorder

import "testing"

func stereo(n int, v float32) []float32 {
	d := make([]float32, n*audioChannels)
	for i := range d {
		d[i] = v
	}
	return d
}

func TestMixerSumsAndClamps(t *testing.T) {
	a, b := newAudioSource("a", 1), newAudioSource("b", 1)
	m := &mixer{sources: []*audioSource{a, b}}
	a.push(0, stereo(100, 0.25), 100)
	b.push(0, stereo(100, 0.9), 100)
	pcm, start := m.take(100)
	if start != 0 || len(pcm) != 200 {
		t.Fatalf("start=%d len=%d", start, len(pcm))
	}
	if pcm[0] != 32767 {
		t.Errorf("0.25+0.9 should clamp to 32767, got %d", pcm[0])
	}
}

func TestMixerFillsGapsWithSilence(t *testing.T) {
	s := newAudioSource("loopback", 1)
	m := &mixer{sources: []*audioSource{s}}
	s.push(0, stereo(480, 0.5), 480)
	// The machine went silent for one second: next packet is far ahead.
	s.push(480+audioRate, stereo(480, 0.5), 480)
	pcm, _ := m.take(int64(960 + audioRate))
	if pcm[0] == 0 || pcm[2*600] != 0 || pcm[len(pcm)-1] == 0 {
		t.Errorf("want sound, silence, sound; got %d %d %d", pcm[0], pcm[2*600], pcm[len(pcm)-1])
	}
	if st := s.finish(int64(960 + audioRate)); st.GapFrames != audioRate || st.Snaps != 0 || st.Continuous {
		t.Errorf("silence is not drift: gap=%d snaps=%d continuous=%v", st.GapFrames, st.Snaps, st.Continuous)
	}
}

func TestMixerToleratesJitter(t *testing.T) {
	s := newAudioSource("mic", 1)
	s.push(0, stereo(480, 0.1), 480)
	s.push(480+100, stereo(480, 0.1), 480) // 2 ms late: within tolerance
	s.push(960-100, stereo(480, 0.1), 480) // 2 ms early
	if st := s.finish(1440); st.Snaps != 0 || st.GapFrames != 0 || st.DroppedFrames != 0 {
		t.Errorf("jitter caused corrections: %+v", st)
	}
}

func TestMixerSnapsFastSource(t *testing.T) {
	s := newAudioSource("mic", 1)
	// A device running fast delivers 2000 more frames than the clock says.
	s.push(0, stereo(audioRate, 0.1), audioRate)
	s.push(audioRate-2000, stereo(4000, 0.1), 4000)
	st := s.finish(audioRate + 2000)
	if st.DroppedFrames != 2000 || st.Snaps != 1 {
		t.Errorf("dropped=%d snaps=%d", st.DroppedFrames, st.Snaps)
	}
	if st.DriftPPM <= 0 {
		t.Errorf("fast device should report positive drift, got %.0f ppm", st.DriftPPM)
	}
}

func TestMixerLateSourceGetsSilenceThenJoins(t *testing.T) {
	s := newAudioSource("mic", 1)
	m := &mixer{sources: []*audioSource{s}}
	m.take(1000) // mixer runs before the source delivered anything
	s.push(500, stereo(1000, 0.5), 1000)
	pcm, start := m.take(1500)
	if start != 1000 || len(pcm) != 1000 || pcm[0] == 0 {
		t.Errorf("start=%d len=%d first=%d", start, len(pcm), pcm[0])
	}
}

func TestMixerDropsPacketsBeforeStart(t *testing.T) {
	s := newAudioSource("loopback", 1)
	s.push(-1000, stereo(500, 0.5), 500)
	s.push(-200, nil, 400) // silent packet straddling the start
	m := &mixer{sources: []*audioSource{s}}
	pcm, _ := m.take(200)
	if len(pcm) != 400 || pcm[0] != 0 {
		t.Errorf("len=%d first=%d", len(pcm), pcm[0])
	}
}
