//go:build windows

package recorder

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Media Foundation sink writer: BGRA frames in, H.264 in an MP4 out. With
// hardware transforms enabled the writer picks the GPU vendor's H.264 MFT
// (NVENC / QuickSync / AMF) and falls back to Microsoft's software encoder.

const (
	mfVersion        = 0x00020070
	mfStartupFull    = 0
	mfInterlaceProg  = 2
	hundredNsPerSec  = 10_000_000
	sinkAddStream    = 3
	sinkSetInputType = 4
	sinkBeginWriting = 5
	sinkWriteSample  = 6
	sinkFinalize     = 11
	attrSetUINT32    = 21
	attrSetUINT64    = 22
	attrSetGUID      = 24
	attrSetUnknown   = 27
	sampleSetTime    = 36
	sampleSetDur     = 38
	sampleAddBuffer  = 42
	bufLock          = 3
	bufUnlock        = 4
	bufSetCurLength  = 6
	buf2DGetContig   = 7
	dxgiMgrReset     = 7
	h264ProfileHigh  = 100
)

var (
	mfMTMajorType          = mustGUID("48EBA18E-F8C9-4687-BF11-0A74C9F96A8F")
	mfMTSubtype            = mustGUID("F7E34C9A-42E8-4714-B74B-CB29D72C35E5")
	mfMTAvgBitrate         = mustGUID("20332624-FB0D-4D9E-BD0D-CBF6786C102E")
	mfMTInterlaceMode      = mustGUID("E2724BB8-E676-4806-B4B2-A8D6EFB44CCD")
	mfMTFrameSize          = mustGUID("1652C33D-D6B2-4012-B834-72030849A37D")
	mfMTFrameRate          = mustGUID("C459A2E8-3D2C-4E44-B132-FEE5156C7BB0")
	mfMTPixelAspectRatio   = mustGUID("C6376A1E-8D0A-4027-BE45-6D9A0AD39BB6")
	mfMTDefaultStride      = mustGUID("644B4E48-1E02-4516-B0EB-C01CA9D49AC6")
	mfMTMpeg2Profile       = mustGUID("AD76A80B-2D5C-4E0B-B375-64E520137036")
	mfReadwriteEnableHWMFT = mustGUID("A634A91C-822B-41B9-A494-4DE4643612B0")
	mfSinkWriterD3DManager = mustGUID("EC822DA2-E1E9-4B29-A0D8-563C719F5269")
	mfMediaTypeVideo       = mustGUID("73646976-0000-0010-8000-00AA00389B71")
	mfVideoFormatH264      = mustGUID("34363248-0000-0010-8000-00AA00389B71")
	mfVideoFormatRGB32     = mustGUID("00000016-0000-0010-8000-00AA00389B71")

	mfMediaTypeAudio        = mustGUID("73647561-0000-0010-8000-00AA00389B71")
	mfAudioFormatAAC        = mustGUID("00001610-0000-0010-8000-00AA00389B71")
	mfAudioFormatPCM        = mustGUID("00000001-0000-0010-8000-00AA00389B71")
	mfMTAudioChannels       = mustGUID("37E48BF5-645E-4C5B-89DE-ADA9E29B696A")
	mfMTAudioSampleRate     = mustGUID("5FAEEAE7-0290-4C31-9E8A-C534F68D9DBA")
	mfMTAudioAvgBytesPerSec = mustGUID("1AAB75C8-CFEF-451C-AB95-AC034B8E1731")
	mfMTAudioBlockAlign     = mustGUID("322DE230-9EEB-43BD-AB7A-FF412251541D")
	mfMTAudioBitsPerSample  = mustGUID("F2DEB57F-40FA-4764-AA33-ED4F2D1FF669")
)

type encoder struct {
	writer   unsafe.Pointer
	devMgr   unsafe.Pointer
	stream   uint32
	audio    int32 // audio stream index, -1 without audio
	w, h     int
	gpu      bool
	frameDur int64
}

// newEncoder opens path for writing. With gpu it hands the D3D11 device to the
// sink writer so frames stay in video memory from capture to encoder.
func newEncoder(path string, w, h, fps, bitrate int, dev *d3dDevice, gpu, audio bool) (e *encoder, err error) {
	if r, _, _ := procMFStartup.Call(mfVersion, mfStartupFull); failed(r) {
		return nil, hresultErr("MFStartup", r)
	}
	e = &encoder{w: w, h: h, gpu: gpu, audio: -1, frameDur: hundredNsPerSec / int64(fps)}
	defer func() {
		if err != nil {
			e.abort()
			e = nil
		}
	}()

	var attrs unsafe.Pointer
	if r, _, _ := procMFCreateAttributes.Call(uintptr(unsafe.Pointer(&attrs)), 2); failed(r) {
		return nil, hresultErr("MFCreateAttributes", r)
	}
	defer release(attrs)
	setUINT32(attrs, &mfReadwriteEnableHWMFT, 1)
	if gpu {
		var token uint32
		if r, _, _ := procMFCreateDXGIDeviceManager.Call(uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&e.devMgr))); failed(r) {
			return nil, hresultErr("MFCreateDXGIDeviceManager", r)
		}
		if r := comCall(e.devMgr, dxgiMgrReset, uintptr(dev.dev), uintptr(token)); failed(r) {
			return nil, hresultErr("IMFDXGIDeviceManager.ResetDevice", r)
		}
		comCall(attrs, attrSetUnknown, uintptr(unsafe.Pointer(&mfSinkWriterD3DManager)), uintptr(e.devMgr))
	}

	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	if r, _, _ := procMFCreateSinkWriterFromURL.Call(uintptr(unsafe.Pointer(pathW)), 0, uintptr(attrs), uintptr(unsafe.Pointer(&e.writer))); failed(r) {
		return nil, hresultErr("MFCreateSinkWriterFromURL", r)
	}

	out, err := videoType(&mfVideoFormatH264, w, h, fps)
	if err != nil {
		return nil, err
	}
	setUINT32(out, &mfMTAvgBitrate, uint32(bitrate))
	setUINT32(out, &mfMTMpeg2Profile, h264ProfileHigh) // default is Baseline
	r := comCall(e.writer, sinkAddStream, uintptr(out), uintptr(unsafe.Pointer(&e.stream)))
	release(out)
	if failed(r) {
		return nil, hresultErr("SinkWriter.AddStream", r)
	}

	in, err := videoType(&mfVideoFormatRGB32, w, h, fps)
	if err != nil {
		return nil, err
	}
	setUINT32(in, &mfMTDefaultStride, uint32(w*4)) // positive: top-down rows
	r = comCall(e.writer, sinkSetInputType, uintptr(e.stream), uintptr(in), 0)
	release(in)
	if failed(r) {
		return nil, hresultErr("SinkWriter.SetInputMediaType", r)
	}
	if audio {
		if err := e.addAudioStream(); err != nil {
			return nil, err
		}
	}
	if r := comCall(e.writer, sinkBeginWriting); failed(r) {
		return nil, hresultErr("SinkWriter.BeginWriting", r)
	}
	return e, nil
}

func videoType(subtype *windows.GUID, w, h, fps int) (unsafe.Pointer, error) {
	var t unsafe.Pointer
	if r, _, _ := procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&t))); failed(r) {
		return nil, hresultErr("MFCreateMediaType", r)
	}
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTMajorType)), uintptr(unsafe.Pointer(&mfMediaTypeVideo)))
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTSubtype)), uintptr(unsafe.Pointer(subtype)))
	setUINT32(t, &mfMTInterlaceMode, mfInterlaceProg)
	setUINT64(t, &mfMTFrameSize, uint64(w)<<32|uint64(h))
	setUINT64(t, &mfMTFrameRate, uint64(fps)<<32|1)
	setUINT64(t, &mfMTPixelAspectRatio, 1<<32|1)
	return t, nil
}

func setUINT32(attrs unsafe.Pointer, key *windows.GUID, v uint32) {
	comCall(attrs, attrSetUINT32, uintptr(unsafe.Pointer(key)), uintptr(v))
}

func setUINT64(attrs unsafe.Pointer, key *windows.GUID, v uint64) {
	comCall(attrs, attrSetUINT64, uintptr(unsafe.Pointer(key)), uintptr(v))
}

// writeTexture encodes a GPU texture without copying it to system memory.
// The sample holds a reference to tex until the encoder is done with it.
func (e *encoder) writeTexture(tex unsafe.Pointer, t int64) error {
	var buf unsafe.Pointer
	if r, _, _ := procMFCreateDXGISurfaceBuffer.Call(uintptr(unsafe.Pointer(&iidID3D11Texture2D)), uintptr(tex), 0, 0,
		uintptr(unsafe.Pointer(&buf))); failed(r) {
		return hresultErr("MFCreateDXGISurfaceBuffer", r)
	}
	defer release(buf)
	if b2d, err := queryInterface(buf, &iidIMF2DBuffer); err == nil {
		var n uint32
		comCall(b2d, buf2DGetContig, uintptr(unsafe.Pointer(&n)))
		comCall(buf, bufSetCurLength, uintptr(n))
		release(b2d)
	}
	return e.writeBuffer(buf, t)
}

// writePixels encodes BGRA rows from system memory.
func (e *encoder) writePixels(data unsafe.Pointer, pitch int, t int64) error {
	size := e.w * e.h * 4
	var buf unsafe.Pointer
	if r, _, _ := procMFCreateMemoryBuffer.Call(uintptr(size), uintptr(unsafe.Pointer(&buf))); failed(r) {
		return hresultErr("MFCreateMemoryBuffer", r)
	}
	defer release(buf)
	var dst unsafe.Pointer
	if r := comCall(buf, bufLock, uintptr(unsafe.Pointer(&dst)), 0, 0); failed(r) {
		return hresultErr("IMFMediaBuffer.Lock", r)
	}
	row := e.w * 4
	dstRows := unsafe.Slice((*byte)(dst), size)
	srcRows := unsafe.Slice((*byte)(data), pitch*(e.h-1)+row)
	for y := 0; y < e.h; y++ {
		copy(dstRows[y*row:(y+1)*row], srcRows[y*pitch:y*pitch+row])
	}
	comCall(buf, bufUnlock)
	comCall(buf, bufSetCurLength, uintptr(size))
	return e.writeBuffer(buf, t)
}

func (e *encoder) writeBuffer(buf unsafe.Pointer, t int64) error {
	var sample unsafe.Pointer
	if r, _, _ := procMFCreateSample.Call(uintptr(unsafe.Pointer(&sample))); failed(r) {
		return hresultErr("MFCreateSample", r)
	}
	defer release(sample)
	comCall(sample, sampleAddBuffer, uintptr(buf))
	comCall(sample, sampleSetTime, uintptr(t))
	comCall(sample, sampleSetDur, uintptr(e.frameDur))
	// Blocks while the encoder is behind: that back-pressure shows up as
	// dropped frame slots in Stats.
	if r := comCall(e.writer, sinkWriteSample, uintptr(e.stream), uintptr(sample)); failed(r) {
		return hresultErr("SinkWriter.WriteSample", r)
	}
	return nil
}

// finish flushes the encoder and writes the MP4 index.
func (e *encoder) finish() error {
	r := comCall(e.writer, sinkFinalize)
	e.abort()
	return hresultErr("SinkWriter.Finalize", r)
}

func (e *encoder) abort() {
	release(e.writer)
	release(e.devMgr)
	e.writer, e.devMgr = nil, nil
	procMFShutdown.Call()
}

const (
	aacBytesPerSec = 24000 // 192 kbit/s, one of the rates the MS AAC encoder accepts
)

func audioType(subtype *windows.GUID) (unsafe.Pointer, error) {
	var t unsafe.Pointer
	if r, _, _ := procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&t))); failed(r) {
		return nil, hresultErr("MFCreateMediaType", r)
	}
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTMajorType)), uintptr(unsafe.Pointer(&mfMediaTypeAudio)))
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTSubtype)), uintptr(unsafe.Pointer(subtype)))
	setUINT32(t, &mfMTAudioBitsPerSample, 16)
	setUINT32(t, &mfMTAudioSampleRate, audioRate)
	setUINT32(t, &mfMTAudioChannels, audioChannels)
	return t, nil
}

// addAudioStream adds the AAC track fed with 48 kHz stereo 16-bit PCM.
func (e *encoder) addAudioStream() error {
	out, err := audioType(&mfAudioFormatAAC)
	if err != nil {
		return err
	}
	setUINT32(out, &mfMTAudioAvgBytesPerSec, aacBytesPerSec)
	var idx uint32
	r := comCall(e.writer, sinkAddStream, uintptr(out), uintptr(unsafe.Pointer(&idx)))
	release(out)
	if failed(r) {
		return hresultErr("SinkWriter.AddStream(AAC)", r)
	}
	in, err := audioType(&mfAudioFormatPCM)
	if err != nil {
		return err
	}
	setUINT32(in, &mfMTAudioBlockAlign, audioChannels*2)
	setUINT32(in, &mfMTAudioAvgBytesPerSec, audioRate*audioChannels*2)
	r = comCall(e.writer, sinkSetInputType, uintptr(idx), uintptr(in), 0)
	release(in)
	if failed(r) {
		return hresultErr("SinkWriter.SetInputMediaType(PCM)", r)
	}
	e.audio = int32(idx)
	return nil
}

// writeAudio encodes interleaved stereo PCM starting at timeline frame start.
func (e *encoder) writeAudio(pcm []int16, start int64) error {
	if e.audio < 0 || len(pcm) == 0 {
		return nil
	}
	size := len(pcm) * 2
	var buf unsafe.Pointer
	if r, _, _ := procMFCreateMemoryBuffer.Call(uintptr(size), uintptr(unsafe.Pointer(&buf))); failed(r) {
		return hresultErr("MFCreateMemoryBuffer", r)
	}
	defer release(buf)
	var dst unsafe.Pointer
	if r := comCall(buf, bufLock, uintptr(unsafe.Pointer(&dst)), 0, 0); failed(r) {
		return hresultErr("IMFMediaBuffer.Lock", r)
	}
	copy(unsafe.Slice((*int16)(dst), len(pcm)), pcm)
	comCall(buf, bufUnlock)
	comCall(buf, bufSetCurLength, uintptr(size))

	var sample unsafe.Pointer
	if r, _, _ := procMFCreateSample.Call(uintptr(unsafe.Pointer(&sample))); failed(r) {
		return hresultErr("MFCreateSample", r)
	}
	defer release(sample)
	frames := int64(len(pcm) / audioChannels)
	comCall(sample, sampleAddBuffer, uintptr(buf))
	comCall(sample, sampleSetTime, uintptr(start*hundredNsPerSec/audioRate))
	comCall(sample, sampleSetDur, uintptr(frames*hundredNsPerSec/audioRate))
	if r := comCall(e.writer, sinkWriteSample, uintptr(e.audio), uintptr(sample)); failed(r) {
		return hresultErr("SinkWriter.WriteSample(audio)", r)
	}
	return nil
}
