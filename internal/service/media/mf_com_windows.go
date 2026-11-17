//go:build windows

package media

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Minimal COM plumbing for Media Foundation: vtable calls, GUIDs and the
// attribute accessors. Vtable indices come from the Windows SDK headers
// (IUnknown's three methods come first). The recorder package has the same
// plumbing for capture; it is unexported there and the packages stay apart.

var (
	mfplat      = windows.NewLazySystemDLL("mfplat.dll")
	mfreadwrite = windows.NewLazySystemDLL("mfreadwrite.dll")

	procMFStartup                   = mfplat.NewProc("MFStartup")
	procMFShutdown                  = mfplat.NewProc("MFShutdown")
	procMFCreateAttributes          = mfplat.NewProc("MFCreateAttributes")
	procMFCreateMediaType           = mfplat.NewProc("MFCreateMediaType")
	procMFCreateSourceReaderFromURL = mfreadwrite.NewProc("MFCreateSourceReaderFromURL")
	procMFCreateSinkWriterFromURL   = mfreadwrite.NewProc("MFCreateSinkWriterFromURL")
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var (
	mfMTMajorType        = mustGUID("48EBA18E-F8C9-4687-BF11-0A74C9F96A8F")
	mfMTSubtype          = mustGUID("F7E34C9A-42E8-4714-B74B-CB29D72C35E5")
	mfMTAvgBitrate       = mustGUID("20332624-FB0D-4D9E-BD0D-CBF6786C102E")
	mfMTInterlaceMode    = mustGUID("E2724BB8-E676-4806-B4B2-A8D6EFB44CCD")
	mfMTFrameSize        = mustGUID("1652C33D-D6B2-4012-B834-72030849A37D")
	mfMTFrameRate        = mustGUID("C459A2E8-3D2C-4E44-B132-FEE5156C7BB0")
	mfMTPixelAspectRatio = mustGUID("C6376A1E-8D0A-4027-BE45-6D9A0AD39BB6")
	mfMTDefaultStride    = mustGUID("644B4E48-1E02-4516-B0EB-C01CA9D49AC6")
	mfMTMpeg2Profile     = mustGUID("AD76A80B-2D5C-4E0B-B375-64E520137036")

	mfMTAudioChannels       = mustGUID("37E48BF5-645E-4C5B-89DE-ADA9E29B696A")
	mfMTAudioSampleRate     = mustGUID("5FAEEAE7-0290-4C31-9E8A-C534F68D9DBA")
	mfMTAudioAvgBytesPerSec = mustGUID("1AAB75C8-CFEF-451C-AB95-AC034B8E1731")
	mfMTAudioBlockAlign     = mustGUID("322DE230-9EEB-43BD-AB7A-FF412251541D")
	mfMTAudioBitsPerSample  = mustGUID("F2DEB57F-40FA-4764-AA33-ED4F2D1FF669")

	mfMediaTypeVideo   = mustGUID("73646976-0000-0010-8000-00AA00389B71")
	mfMediaTypeAudio   = mustGUID("73647561-0000-0010-8000-00AA00389B71")
	mfVideoFormatH264  = mustGUID("34363248-0000-0010-8000-00AA00389B71")
	mfVideoFormatRGB32 = mustGUID("00000016-0000-0010-8000-00AA00389B71")
	mfVideoFormatNV12  = mustGUID("3231564E-0000-0010-8000-00AA00389B71")
	mfAudioFormatAAC   = mustGUID("00001610-0000-0010-8000-00AA00389B71")
	mfAudioFormatPCM   = mustGUID("00000001-0000-0010-8000-00AA00389B71")

	mfReadwriteEnableHWMFT        = mustGUID("A634A91C-822B-41B9-A494-4DE4643612B0")
	mfSourceReaderEnableVideoProc = mustGUID("FB394F3D-CCF1-42EE-BBB3-F9B845D5681D")
	// MF_SOURCE_READER_ENABLE_ADVANCED_VIDEO_PROCESSING: like the basic one, and
	// it can also resize the decoded frames.
	mfSourceReaderEnableAdvancedVideoProc = mustGUID("0F81DA2C-B537-4672-A8B2-A681B17307A3")
	mfPDDuration                  = mustGUID("6C990D33-BB8E-477A-8598-0D5D96FCD88A")
	iidIMF2DBuffer                = mustGUID("7DC9D5F9-9ED9-44EC-9BBF-0600BB589FBB")
)

const (
	mfVersion       = 0x00020070
	mfStartupFull   = 0
	hundredNsPerSec = 10_000_000
	mfInterlaceProg = 2

	// IMFAttributes
	attrGetUINT32 = 7
	attrGetUINT64 = 8
	attrGetGUID   = 10
	attrSetUINT32 = 21
	attrSetUINT64 = 22
	attrSetGUID   = 24
	// IMFSample (after IMFAttributes)
	sampleGetTime       = 35
	sampleSetTime       = 36
	sampleGetDuration   = 37
	sampleSetDuration   = 38
	sampleConvertToCont = 41
	// IMFMediaBuffer
	bufLock   = 3
	bufUnlock = 4
	// IMF2DBuffer
	buf2DLock   = 3
	buf2DUnlock = 4
	// IMFSourceReader
	srSetStreamSelection  = 4
	srGetNativeType       = 5
	srGetCurrentType      = 6
	srSetCurrentType      = 7
	srSetCurrentPosition  = 8
	srReadSample          = 9
	srGetPresentationAttr = 12
	// IMFSinkWriter
	sinkAddStream    = 3
	sinkSetInputType = 4
	sinkBeginWriting = 5
	sinkWriteSample  = 6
	sinkFinalize     = 11

	srAllStreams  = 0xFFFFFFFE // also ANY_STREAM for ReadSample
	srMediaSource = 0xFFFFFFFF

	readerFlagError       = 0x1
	readerFlagEndOfStream = 0x2

	mfEInvalidStreamNumber = 0xC00D36B3
	vtI8, vtUI8            = 20, 21
	h264ProfileHigh        = 100
)

// comCall invokes method idx of the COM object obj.
//
// go:uintptrescapes keeps pointers converted to uintptr at the call site
// alive and on the heap until the call returns, as for windows.Proc.Call.
//
//go:uintptrescapes
func comCall(obj unsafe.Pointer, idx int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(obj))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

func failed(r uintptr) bool { return int32(r) < 0 }

func hresultErr(what string, r uintptr) error {
	if !failed(r) {
		return nil
	}
	return fmt.Errorf("%s: HRESULT 0x%08X", what, uint32(r))
}

func release(obj unsafe.Pointer) {
	if obj != nil {
		comCall(obj, 2)
	}
}

func queryInterface(obj unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	var out unsafe.Pointer
	if r := comCall(obj, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))); failed(r) {
		return nil, hresultErr("QueryInterface", r)
	}
	return out, nil
}

// mfStartup initialises Media Foundation; the returned func shuts it down.
func mfStartup() (func(), error) {
	if r, _, _ := procMFStartup.Call(mfVersion, mfStartupFull); failed(r) {
		return nil, hresultErr("MFStartup", r)
	}
	return func() { procMFShutdown.Call() }, nil
}

func newAttributes() (unsafe.Pointer, error) {
	var a unsafe.Pointer
	if r, _, _ := procMFCreateAttributes.Call(uintptr(unsafe.Pointer(&a)), 2); failed(r) {
		return nil, hresultErr("MFCreateAttributes", r)
	}
	return a, nil
}

func setUINT32(attrs unsafe.Pointer, key *windows.GUID, v uint32) {
	comCall(attrs, attrSetUINT32, uintptr(unsafe.Pointer(key)), uintptr(v))
}

func setUINT64(attrs unsafe.Pointer, key *windows.GUID, v uint64) {
	comCall(attrs, attrSetUINT64, uintptr(unsafe.Pointer(key)), uintptr(v))
}

func getUINT32(attrs unsafe.Pointer, key *windows.GUID) (uint32, bool) {
	var v uint32
	r := comCall(attrs, attrGetUINT32, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&v)))
	return v, !failed(r)
}

func getUINT64(attrs unsafe.Pointer, key *windows.GUID) (uint64, bool) {
	var v uint64
	r := comCall(attrs, attrGetUINT64, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&v)))
	return v, !failed(r)
}

func getGUID(attrs unsafe.Pointer, key *windows.GUID) (windows.GUID, bool) {
	var v windows.GUID
	r := comCall(attrs, attrGetGUID, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&v)))
	return v, !failed(r)
}

// newType makes a media type with a major type and a subtype.
func newType(major, subtype *windows.GUID) (unsafe.Pointer, error) {
	var t unsafe.Pointer
	if r, _, _ := procMFCreateMediaType.Call(uintptr(unsafe.Pointer(&t))); failed(r) {
		return nil, hresultErr("MFCreateMediaType", r)
	}
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTMajorType)), uintptr(unsafe.Pointer(major)))
	comCall(t, attrSetGUID, uintptr(unsafe.Pointer(&mfMTSubtype)), uintptr(unsafe.Pointer(subtype)))
	return t, nil
}

// videoType is a progressive video type of the given size and frame rate.
func videoType(subtype *windows.GUID, w, h int, rateNum, rateDen uint32) (unsafe.Pointer, error) {
	t, err := newType(&mfMediaTypeVideo, subtype)
	if err != nil {
		return nil, err
	}
	setUINT32(t, &mfMTInterlaceMode, mfInterlaceProg)
	setUINT64(t, &mfMTFrameSize, uint64(w)<<32|uint64(h))
	setUINT64(t, &mfMTFrameRate, uint64(rateNum)<<32|uint64(rateDen))
	setUINT64(t, &mfMTPixelAspectRatio, 1<<32|1)
	return t, nil
}

// audioType is an audio type; the caller sets the format-specific fields.
func audioType(subtype *windows.GUID, rate, channels uint32) (unsafe.Pointer, error) {
	t, err := newType(&mfMediaTypeAudio, subtype)
	if err != nil {
		return nil, err
	}
	setUINT32(t, &mfMTAudioBitsPerSample, 16)
	setUINT32(t, &mfMTAudioSampleRate, rate)
	setUINT32(t, &mfMTAudioChannels, channels)
	return t, nil
}

// sourceReader wraps IMFSourceReader.
type sourceReader struct{ p unsafe.Pointer }

// openReader opens path. With videoProc the reader can convert decoded YUV to
// RGB32 itself.
func openReader(path string, videoProc bool) (*sourceReader, error) {
	return openReaderWith(path, videoProc, false)
}

// openReaderScaling opens a reader whose video output can have another frame
// size than the source (advanced video processing).
func openReaderScaling(path string) (*sourceReader, error) {
	return openReaderWith(path, false, true)
}

func openReaderWith(path string, videoProc, advanced bool) (*sourceReader, error) {
	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrs, err := newAttributes()
	if err != nil {
		return nil, err
	}
	defer release(attrs)
	if videoProc {
		setUINT32(attrs, &mfSourceReaderEnableVideoProc, 1)
	}
	if advanced {
		setUINT32(attrs, &mfSourceReaderEnableAdvancedVideoProc, 1)
	}
	var r unsafe.Pointer
	if hr, _, _ := procMFCreateSourceReaderFromURL.Call(uintptr(unsafe.Pointer(pathW)), uintptr(attrs), uintptr(unsafe.Pointer(&r))); failed(hr) {
		return nil, hresultErr("MFCreateSourceReaderFromURL", hr)
	}
	return &sourceReader{p: r}, nil
}

func (r *sourceReader) close() {
	release(r.p)
	r.p = nil
}

// streams returns the reader stream index of the first video and audio
// stream, or -1.
func (r *sourceReader) streams() (video, audio int) {
	video, audio = -1, -1
	for i := 0; i < 64; i++ {
		var t unsafe.Pointer
		hr := comCall(r.p, srGetNativeType, uintptr(i), 0, uintptr(unsafe.Pointer(&t)))
		if uint32(hr) == mfEInvalidStreamNumber {
			break
		}
		if failed(hr) {
			continue
		}
		if major, ok := getGUID(t, &mfMTMajorType); ok {
			switch major {
			case mfMediaTypeVideo:
				if video < 0 {
					video = i
				}
			case mfMediaTypeAudio:
				if audio < 0 {
					audio = i
				}
			}
		}
		release(t)
	}
	return video, audio
}

// selectOnly turns every stream off except the listed ones.
func (r *sourceReader) selectOnly(streams ...int) {
	comCall(r.p, srSetStreamSelection, srAllStreams, 0)
	for _, s := range streams {
		if s >= 0 {
			comCall(r.p, srSetStreamSelection, uintptr(s), 1)
		}
	}
}

func (r *sourceReader) nativeType(stream int) (unsafe.Pointer, error) {
	var t unsafe.Pointer
	if hr := comCall(r.p, srGetNativeType, uintptr(stream), 0, uintptr(unsafe.Pointer(&t))); failed(hr) {
		return nil, hresultErr("GetNativeMediaType", hr)
	}
	return t, nil
}

// setType asks the reader to decode the stream to the type t.
func (r *sourceReader) setType(stream int, t unsafe.Pointer) error {
	return hresultErr("SetCurrentMediaType", comCall(r.p, srSetCurrentType, uintptr(stream), 0, uintptr(t)))
}

func (r *sourceReader) currentType(stream int) (unsafe.Pointer, error) {
	var t unsafe.Pointer
	if hr := comCall(r.p, srGetCurrentType, uintptr(stream), uintptr(unsafe.Pointer(&t))); failed(hr) {
		return nil, hresultErr("GetCurrentMediaType", hr)
	}
	return t, nil
}

// seek moves every selected stream to t (100 ns units). The reader lands on
// the keyframe at or before t, so the first samples can be earlier.
func (r *sourceReader) seek(t int64) error {
	// A PROPVARIANT is 24 bytes on 64-bit Windows: type, padding, 16-byte union.
	pv := [3]uint64{vtI8, uint64(t), 0}
	var zero windows.GUID // GUID_NULL: position in 100 ns units
	return hresultErr("SetCurrentPosition",
		comCall(r.p, srSetCurrentPosition, uintptr(unsafe.Pointer(&zero)), uintptr(unsafe.Pointer(&pv[0]))))
}

// duration is the length of the presentation in 100 ns units.
func (r *sourceReader) duration() (int64, error) {
	var pv [3]uint64
	hr := comCall(r.p, srGetPresentationAttr, srMediaSource, uintptr(unsafe.Pointer(&mfPDDuration)), uintptr(unsafe.Pointer(&pv[0])))
	if failed(hr) {
		return 0, hresultErr("GetPresentationAttribute(duration)", hr)
	}
	if pv[0]&0xFFFF != vtUI8 && pv[0]&0xFFFF != vtI8 {
		return 0, fmt.Errorf("duration has an unexpected type %d", pv[0]&0xFFFF)
	}
	return int64(pv[1]), nil
}

// readResult is one ReadSample outcome. sample is nil for a gap, a type
// change or the end of the stream; the caller releases a non-nil sample.
type readResult struct {
	sample unsafe.Pointer
	stream int
	eos    bool
	time   int64
}

func (r *sourceReader) read(stream uint32) (readResult, error) {
	var actual, flags uint32
	var ts int64
	var s unsafe.Pointer
	hr := comCall(r.p, srReadSample, uintptr(stream), 0,
		uintptr(unsafe.Pointer(&actual)), uintptr(unsafe.Pointer(&flags)),
		uintptr(unsafe.Pointer(&ts)), uintptr(unsafe.Pointer(&s)))
	if failed(hr) {
		return readResult{}, hresultErr("ReadSample", hr)
	}
	if flags&readerFlagError != 0 {
		release(s)
		return readResult{}, errors.New("ReadSample: stream error")
	}
	return readResult{sample: s, stream: int(actual), eos: flags&readerFlagEndOfStream != 0, time: ts}, nil
}

func sampleTime(s unsafe.Pointer) int64 {
	var t int64
	comCall(s, sampleGetTime, uintptr(unsafe.Pointer(&t)))
	return t
}

func sampleDuration(s unsafe.Pointer) int64 {
	var d int64
	comCall(s, sampleGetDuration, uintptr(unsafe.Pointer(&d)))
	return d
}

func frameSize(t unsafe.Pointer) (w, h int, err error) {
	v, ok := getUINT64(t, &mfMTFrameSize)
	if !ok || v>>32 == 0 || uint32(v) == 0 {
		return 0, 0, errors.New("media type has no frame size")
	}
	return int(v >> 32), int(uint32(v)), nil
}
