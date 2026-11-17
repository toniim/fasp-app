//go:build windows

package media

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows backend: the Media Foundation Source Reader decodes (for the GIF
// frames and the trim), the Sink Writer encodes the trim as H.264 + AAC in
// an MP4, the way the recorder does.

func probeNative(path string) (time.Duration, int, int, error) {
	shutdown, err := mfStartup()
	if err != nil {
		return 0, 0, 0, err
	}
	defer shutdown()
	r, err := openReader(path, false)
	if err != nil {
		return 0, 0, 0, err
	}
	defer r.close()
	v, _ := r.streams()
	if v < 0 {
		return 0, 0, 0, errors.New("no video track")
	}
	t, err := r.nativeType(v)
	if err != nil {
		return 0, 0, 0, err
	}
	defer release(t)
	w, h, err := frameSize(t)
	if err != nil {
		return 0, 0, 0, err
	}
	d, err := r.duration()
	if err != nil {
		return 0, 0, 0, err
	}
	if d <= 0 {
		return 0, 0, 0, errors.New("unknown duration")
	}
	return time.Duration(d) * 100, w, h, nil
}

// frameReader walks the video samples of a reader in presentation order with
// one sample of look-ahead, so the sample that is current at a time t is known.
type frameReader struct {
	r      *sourceReader
	stream int
	next   unsafe.Pointer // read ahead, owned
	nextT  int64
	eos    bool
}

// advance reads the next video sample into next (which must be nil).
func (f *frameReader) advance() error {
	for !f.eos {
		res, err := f.r.read(uint32(f.stream))
		if err != nil {
			return err
		}
		if res.eos {
			f.eos = true
		}
		if res.sample != nil {
			f.next, f.nextT = res.sample, res.time
			return nil
		}
	}
	return nil
}

// decodeFrames calls fn with count BGRA frames of size w x h, sampled at
// start + k/fps, the last decoded frame at or before each sample time. The
// slice is only valid during the call.
func decodeFrames(in string, start time.Duration, count int, fps float64, w, h int, fn func(i int, bgra []byte) error) error {
	shutdown, err := mfStartup()
	if err != nil {
		return err
	}
	defer shutdown()
	r, err := openReader(in, true) // video processing converts YUV to RGB32
	if err != nil {
		return err
	}
	defer r.close()
	v, _ := r.streams()
	if v < 0 {
		return errors.New("no video track")
	}
	r.selectOnly(v)

	want, err := newType(&mfMediaTypeVideo, &mfVideoFormatRGB32)
	if err != nil {
		return err
	}
	err = r.setType(v, want)
	release(want)
	if err != nil {
		return err
	}
	got, err := r.currentType(v)
	if err != nil {
		return err
	}
	sw, sh, err := frameSize(got)
	stride, hasStride := getUINT32(got, &mfMTDefaultStride)
	release(got)
	if err != nil {
		return err
	}
	rowStride := sw * 4
	if hasStride && int32(stride) != 0 {
		rowStride = int(int32(stride))
	}

	t0 := int64(start) / 100
	if t0 > 0 {
		if err := r.seek(t0); err != nil {
			return err
		}
	}

	fr := &frameReader{r: r, stream: v}
	var cur unsafe.Pointer
	defer func() {
		release(cur)
		release(fr.next)
	}()
	if err := fr.advance(); err != nil {
		return err
	}

	sc := newScaler(sw, sh, w, h)
	var out []byte
	step := float64(hundredNsPerSec) / fps
	for k := 0; k < count; k++ {
		t := t0 + int64(float64(k)*step)
		changed := false
		for fr.next != nil && fr.nextT <= t+5000 { // 0.5 ms of slack
			release(cur)
			cur, fr.next = fr.next, nil
			changed = true
			if err := fr.advance(); err != nil {
				return err
			}
		}
		if cur == nil { // the first frame comes after the requested start
			cur, fr.next = fr.next, nil
			changed = true
			if cur != nil {
				if err := fr.advance(); err != nil {
					return err
				}
			}
		}
		if cur == nil {
			return errors.New("no video frame to read")
		}
		if changed { // a still screen repeats the frame without converting it again
			if err := readRGB32(cur, sc, sw, sh, rowStride); err != nil {
				return err
			}
			out = sc.scale()
		}
		if err := fn(k, out); err != nil {
			return err
		}
	}
	return nil
}

// readRGB32 copies the pixels of a sample into the scaler, top row first.
func readRGB32(sample unsafe.Pointer, sc *scaler, w, h, stride int) error {
	var buf unsafe.Pointer
	if hr := comCall(sample, sampleConvertToCont, uintptr(unsafe.Pointer(&buf))); failed(hr) {
		return hresultErr("ConvertToContiguousBuffer", hr)
	}
	defer release(buf)
	rowBytes := w * 4

	// IMF2DBuffer gives the top row and a signed pitch, which is right for both
	// bottom-up and top-down buffers.
	if b2d, err := queryInterface(buf, &iidIMF2DBuffer); err == nil {
		defer release(b2d)
		var scan0 unsafe.Pointer
		var pitch int32
		if hr := comCall(b2d, buf2DLock, uintptr(unsafe.Pointer(&scan0)), uintptr(unsafe.Pointer(&pitch))); !failed(hr) {
			defer comCall(b2d, buf2DUnlock)
			for y := 0; y < h; y++ {
				copy(sc.row(y), unsafe.Slice((*byte)(unsafe.Add(scan0, y*int(pitch))), rowBytes))
			}
			return nil
		}
	}

	var data unsafe.Pointer
	var maxLen, curLen uint32
	if hr := comCall(buf, bufLock, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&maxLen)), uintptr(unsafe.Pointer(&curLen))); failed(hr) {
		return hresultErr("IMFMediaBuffer.Lock", hr)
	}
	defer comCall(buf, bufUnlock)
	pitch := stride
	if pitch < 0 {
		pitch = -pitch // negative default stride: bottom-up, first row in memory is the bottom
	}
	if pitch < rowBytes || int(curLen) < pitch*(h-1)+rowBytes {
		return fmt.Errorf("video buffer of %d bytes is too small for %dx%d", curLen, w, h)
	}
	for y := 0; y < h; y++ {
		src := y
		if stride < 0 {
			src = h - 1 - y
		}
		copy(sc.row(y), unsafe.Slice((*byte)(unsafe.Add(data, src*pitch)), rowBytes))
	}
	return nil
}

// trimNative re-encodes [start, end) of in as H.264 + AAC. It tries the
// hardware encoder first and the software one when that fails, as the
// recorder does.
func trimNative(in, out string, start, end time.Duration) error {
	err := trim(in, out, start, end, true)
	if err == nil {
		return nil
	}
	if err2 := trim(in, out, start, end, false); err2 != nil {
		return fmt.Errorf("%w (software encoder: %v)", err, err2)
	}
	return nil
}

func trim(in, out string, start, end time.Duration, hardware bool) error {
	shutdown, err := mfStartup()
	if err != nil {
		return err
	}
	defer shutdown()
	r, err := openReader(in, false)
	if err != nil {
		return err
	}
	defer r.close()
	v, a := r.streams()
	if v < 0 {
		return errors.New("no video track")
	}
	r.selectOnly(v, a)

	// Decode the video to NV12, the encoder's own input format, so frames go
	// through without a colour conversion.
	vt, err := newType(&mfMediaTypeVideo, &mfVideoFormatNV12)
	if err != nil {
		return err
	}
	err = r.setType(v, vt)
	release(vt)
	if err != nil {
		return err
	}
	vcur, err := r.currentType(v)
	if err != nil {
		return err
	}
	w, h, err := frameSize(vcur)
	rate, ok := getUINT64(vcur, &mfMTFrameRate)
	release(vcur)
	if err != nil {
		return err
	}
	rateNum, rateDen := uint32(rate>>32), uint32(rate)
	if !ok || rateNum == 0 || rateDen == 0 {
		rateNum, rateDen = 30, 1
	}

	// Media Foundation's H.264 encoder refuses frames above its limit
	// (MF_E_INVALIDMEDIATYPE at SetInputMediaType), e.g. a 4800x2700 Retina
	// recording: read such a source through a reader that scales it to fit.
	if fw, fh := fitEncoder(w, h); fw != w || fh != h {
		r.close()
		if r, err = openReaderScaling(in); err != nil {
			return err
		}
		defer r.close()
		r.selectOnly(v, a)
		st, err := videoType(&mfVideoFormatNV12, fw, fh, rateNum, rateDen)
		if err != nil {
			return err
		}
		err = r.setType(v, st)
		release(st)
		if err != nil {
			return fmt.Errorf("scale %dx%d to %dx%d for the encoder: %w", w, h, fw, fh, err)
		}
		w, h = fw, fh
	}

	var audioRate, audioCh uint32
	if a >= 0 {
		if audioRate, audioCh, err = decodeAudioToPCM(r, a); err != nil {
			return err
		}
	}

	wr, err := createWriter(out, hardware)
	if err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished { // an unfinished writer still holds the file open
			comCall(wr, sinkFinalize)
		}
		release(wr)
	}()

	fps := float64(rateNum) / float64(rateDen)
	bitrate := int64(w) * int64(h) * int64(min(max(fps, 1), 60)) / 10
	bitrate = min(max(bitrate, 1_000_000), 40_000_000)
	vOut, err := addVideoStream(wr, w, h, rateNum, rateDen, uint32(bitrate))
	if err != nil {
		return err
	}
	aOut := -1
	if a >= 0 {
		if aOut, err = addAudioStream(wr, audioRate, audioCh); err != nil {
			return err
		}
	}
	if hr := comCall(wr, sinkBeginWriting); failed(hr) {
		return hresultErr("SinkWriter.BeginWriting", hr)
	}

	if err := copyRange(r, wr, v, a, vOut, aOut, int64(start)/100, int64(end)/100); err != nil {
		return err
	}
	finished = true
	return hresultErr("SinkWriter.Finalize", comCall(wr, sinkFinalize))
}

// The largest frame Media Foundation's H.264 encoder accepts (level 5.1/5.2).
const maxEncodeW, maxEncodeH = 4096, 2304

// fitEncoder returns w x h scaled down, aspect kept and even, to fit the
// encoder's limit; a size that fits is returned as is.
func fitEncoder(w, h int) (int, int) {
	if w <= maxEncodeW && h <= maxEncodeH {
		return w, h
	}
	k := min(float64(maxEncodeW)/float64(w), float64(maxEncodeH)/float64(h))
	return int(float64(w)*k) &^ 1, int(float64(h)*k) &^ 1
}

// decodeAudioToPCM asks the reader for 16-bit PCM in the stream's own rate and
// channel count, which the AAC encoder takes as is.
func decodeAudioToPCM(r *sourceReader, a int) (rate, channels uint32, err error) {
	native, err := r.nativeType(a)
	if err != nil {
		return 0, 0, err
	}
	rate, _ = getUINT32(native, &mfMTAudioSampleRate)
	channels, _ = getUINT32(native, &mfMTAudioChannels)
	release(native)
	// The AAC encoder takes 44.1 or 48 kHz, mono or stereo.
	if (rate != 44100 && rate != 48000) || (channels != 1 && channels != 2) {
		return 0, 0, fmt.Errorf("unsupported audio format: %d Hz, %d channels", rate, channels)
	}
	pcm, err := audioType(&mfAudioFormatPCM, rate, channels)
	if err != nil {
		return 0, 0, err
	}
	err = r.setType(a, pcm)
	release(pcm)
	return rate, channels, err
}

func createWriter(path string, hardware bool) (unsafe.Pointer, error) {
	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrs, err := newAttributes()
	if err != nil {
		return nil, err
	}
	defer release(attrs)
	if hardware {
		setUINT32(attrs, &mfReadwriteEnableHWMFT, 1)
	}
	var wr unsafe.Pointer
	if hr, _, _ := procMFCreateSinkWriterFromURL.Call(uintptr(unsafe.Pointer(pathW)), 0, uintptr(attrs), uintptr(unsafe.Pointer(&wr))); failed(hr) {
		return nil, hresultErr("MFCreateSinkWriterFromURL", hr)
	}
	return wr, nil
}

func addVideoStream(wr unsafe.Pointer, w, h int, rateNum, rateDen, bitrate uint32) (int, error) {
	out, err := videoType(&mfVideoFormatH264, w, h, rateNum, rateDen)
	if err != nil {
		return 0, err
	}
	setUINT32(out, &mfMTAvgBitrate, bitrate)
	setUINT32(out, &mfMTMpeg2Profile, h264ProfileHigh) // default is Baseline
	var idx uint32
	hr := comCall(wr, sinkAddStream, uintptr(out), uintptr(unsafe.Pointer(&idx)))
	release(out)
	if failed(hr) {
		return 0, hresultErr("SinkWriter.AddStream(H.264)", hr)
	}
	in, err := videoType(&mfVideoFormatNV12, w, h, rateNum, rateDen)
	if err != nil {
		return 0, err
	}
	hr = comCall(wr, sinkSetInputType, uintptr(idx), uintptr(in), 0)
	release(in)
	if failed(hr) {
		return 0, hresultErr("SinkWriter.SetInputMediaType(NV12)", hr)
	}
	return int(idx), nil
}

func addAudioStream(wr unsafe.Pointer, rate, channels uint32) (int, error) {
	out, err := audioType(&mfAudioFormatAAC, rate, channels)
	if err != nil {
		return 0, err
	}
	setUINT32(out, &mfMTAudioAvgBytesPerSec, 24000) // 192 kbit/s, a rate the MS AAC encoder accepts
	var idx uint32
	hr := comCall(wr, sinkAddStream, uintptr(out), uintptr(unsafe.Pointer(&idx)))
	release(out)
	if failed(hr) {
		return 0, hresultErr("SinkWriter.AddStream(AAC)", hr)
	}
	in, err := audioType(&mfAudioFormatPCM, rate, channels)
	if err != nil {
		return 0, err
	}
	setUINT32(in, &mfMTAudioBlockAlign, channels*2)
	setUINT32(in, &mfMTAudioAvgBytesPerSec, rate*channels*2)
	hr = comCall(wr, sinkSetInputType, uintptr(idx), uintptr(in), 0)
	release(in)
	if failed(hr) {
		return 0, hresultErr("SinkWriter.SetInputMediaType(PCM)", hr)
	}
	return int(idx), nil
}

// copyRange moves the samples between startT and endT (100 ns units) from the
// reader to the writer, with times shifted so the output starts at 0. Video
// and audio take the same shift, which keeps them in sync.
func copyRange(r *sourceReader, wr unsafe.Pointer, v, a, vOut, aOut int, startT, endT int64) error {
	if startT > 0 {
		if err := r.seek(startT); err != nil {
			return err
		}
	}
	write := func(stream int, s unsafe.Pointer, t, dur int64) error {
		comCall(s, sampleSetTime, uintptr(t))
		if dur > 0 {
			comCall(s, sampleSetDuration, uintptr(dur))
		}
		return hresultErr("SinkWriter.WriteSample", comCall(wr, sinkWriteSample, uintptr(stream), uintptr(s)))
	}

	// The seek lands on a keyframe before startT. Video before startT is not
	// written, but the last such frame is what the viewer sees at startT, so it
	// is kept and written at time 0 if no frame starts exactly there.
	var held unsafe.Pointer
	defer func() { release(held) }()
	videoDone, audioDone := false, a < 0
	for !videoDone || !audioDone {
		res, err := r.read(srAllStreams)
		if err != nil {
			return err
		}
		if res.eos {
			switch res.stream {
			case v:
				videoDone = true
			case a:
				audioDone = true
			}
		}
		s := res.sample
		if s == nil {
			continue
		}
		switch res.stream {
		case v:
			switch {
			case videoDone || res.time >= endT:
				videoDone = true
			case res.time < startT:
				release(held)
				held = s
				continue
			default:
				if held != nil {
					if gap := res.time - startT; gap > 10_000 { // more than 1 ms
						if err := write(vOut, held, 0, gap); err != nil {
							release(s)
							return err
						}
					}
					release(held)
					held = nil
				}
				err := write(vOut, s, res.time-startT, 0)
				release(s)
				if err != nil {
					return err
				}
				continue
			}
		case a:
			switch {
			case audioDone || res.time >= endT:
				audioDone = true
			case res.time < startT:
				// An audio sample is about 20 ms; dropping the one that straddles
				// startT costs less than a frame of sync.
			default:
				err := write(aOut, s, res.time-startT, 0)
				release(s)
				if err != nil {
					return err
				}
				continue
			}
		}
		release(s)
	}
	if held != nil { // no video frame starts inside the range: show the held one throughout
		if err := write(vOut, held, 0, endT-startT); err != nil {
			return err
		}
	}
	return nil
}
