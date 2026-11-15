//go:build windows

package recorder

import (
	"errors"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WASAPI shared-mode capture. System audio is a loopback stream on the
// default render device (the whole machine, as decided for the product); the
// microphone is a normal capture stream. AUTOCONVERTPCM makes Windows convert
// every device to 48 kHz stereo float, so the mixer never resamples formats.

var (
	ole32    = windows.NewLazySystemDLL("ole32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCoInitializeEx            = ole32.NewProc("CoInitializeEx")
	procCoCreateInstance          = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree             = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear          = ole32.NewProc("PropVariantClear")
	procQueryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")

	clsidMMDeviceEnumerator = mustGUID("BCDE0395-E52F-467C-8E3D-C4579291692E")
	iidIMMDeviceEnumerator  = mustGUID("A95664D2-9614-4F35-A746-DE8DB63617E6")
	iidIAudioClient         = mustGUID("1CB9AD4C-DBFA-4C32-B178-C2F568A703B2")
	iidIAudioCaptureClient  = mustGUID("C8ADBD64-E71E-48A0-A4DE-185C395CD317")
	ksSubtypeIEEEFloat      = mustGUID("00000003-0000-0010-8000-00AA00389B71")
	pkeyDeviceFriendlyName  = propertyKey{mustGUID("A45C254E-DF1C-4EFD-8020-67D146A850E0"), 14}
)

const (
	coinitMultithreaded = 0
	clsctxAll           = 0x17
	eRender, eCapture   = 0, 1
	eConsole            = 0
	deviceStateActive   = 1

	audclntShareShared      = 0
	audclntLoopback         = 0x00020000
	audclntAutoConvertPCM   = 0x80000000
	audclntSrcDefaultQualty = 0x08000000
	audclntBufferSilent     = 0x2
	audclntDeviceInvalid    = 0x88890004

	enumEnumAudioEndpoints = 3
	enumGetDefaultEndpoint = 4
	enumGetDevice          = 5
	devActivate            = 3
	devOpenPropertyStore   = 4
	devGetID               = 5
	collGetCount           = 3
	collItem               = 4
	propsGetValue          = 5
	acInitialize           = 3
	acStart                = 10
	acStop                 = 11
	acGetService           = 14
	accGetBuffer           = 3
	accReleaseBuffer       = 4
	accGetNextPacketSize   = 5

	audioBuffer100ns = 2_000_000 // 200 ms device buffer
)

type propertyKey struct {
	FmtID windows.GUID
	PID   uint32
}

type propVariant struct {
	VT       uint16
	_        [3]uint16
	Val      uintptr
	_padding uintptr
}

type waveFormatExtensible struct {
	FormatTag, Channels           uint16
	SamplesPerSec, AvgBytesPerSec uint32
	BlockAlign, BitsPerSample     uint16
	CbSize, ValidBitsPerSample    uint16
	ChannelMask                   uint32
	SubFormat                     windows.GUID
}

// qpc100ns is the system's QueryPerformanceCounter in 100 ns units: the clock
// WGC frame times and WASAPI packet times are both based on.
func qpc100ns() int64 {
	var c, f int64
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&c)))
	procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&f)))
	return c/f*hundredNsPerSec + c%f*hundredNsPerSec/f
}

func deviceEnumerator() (unsafe.Pointer, error) {
	var e unsafe.Pointer
	if r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&e))); failed(r) {
		return nil, hresultErr("CoCreateInstance(MMDeviceEnumerator)", r)
	}
	return e, nil
}

// ListMicrophones returns the active capture devices.
func ListMicrophones() ([]AudioDevice, error) {
	var out []AudioDevice
	err := withCOM(func() error {
		en, err := deviceEnumerator()
		if err != nil {
			return err
		}
		defer release(en)
		var coll unsafe.Pointer
		if r := comCall(en, enumEnumAudioEndpoints, eCapture, deviceStateActive, uintptr(unsafe.Pointer(&coll))); failed(r) {
			return hresultErr("EnumAudioEndpoints", r)
		}
		defer release(coll)
		var n uint32
		comCall(coll, collGetCount, uintptr(unsafe.Pointer(&n)))
		for i := uint32(0); i < n; i++ {
			var dev unsafe.Pointer
			if r := comCall(coll, collItem, uintptr(i), uintptr(unsafe.Pointer(&dev))); failed(r) {
				continue
			}
			out = append(out, AudioDevice{ID: deviceID(dev), Name: deviceName(dev)})
			release(dev)
		}
		return nil
	})
	return out, err
}

func deviceID(dev unsafe.Pointer) string {
	var p *uint16
	if r := comCall(dev, devGetID, uintptr(unsafe.Pointer(&p))); failed(r) || p == nil {
		return ""
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return windows.UTF16PtrToString(p)
}

func deviceName(dev unsafe.Pointer) string {
	var store unsafe.Pointer
	if r := comCall(dev, devOpenPropertyStore, 0, uintptr(unsafe.Pointer(&store))); failed(r) {
		return ""
	}
	defer release(store)
	var pv propVariant
	if r := comCall(store, propsGetValue, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv))); failed(r) {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	const vtLPWSTR = 31
	if pv.VT != vtLPWSTR || pv.Val == 0 {
		return ""
	}
	return windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&pv.Val)))
}

// withCOM runs fn on a locked OS thread with COM initialized.
func withCOM(fn func() error) error {
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		procCoInitializeEx.Call(0, coinitMultithreaded)
		errc <- fn()
	}()
	return <-errc
}

// wasapiCapture feeds one audioSource from a device until stopped.
type wasapiCapture struct {
	src      *audioSource
	loopback bool
	deviceID string // "" = default device, followed when it changes
	origin   int64  // qpc100ns of timeline frame 0
	stop     chan struct{}
	wg       sync.WaitGroup
	reopens  int
	err      error
}

type wasapiStream struct {
	client, capture unsafe.Pointer
}

func (s *wasapiStream) close() {
	if s.client != nil {
		comCall(s.client, acStop)
	}
	release(s.capture)
	release(s.client)
}

func (c *wasapiCapture) open() (*wasapiStream, error) {
	en, err := deviceEnumerator()
	if err != nil {
		return nil, err
	}
	defer release(en)
	var dev unsafe.Pointer
	if c.deviceID == "" {
		flow := uintptr(eCapture)
		if c.loopback {
			flow = eRender
		}
		if r := comCall(en, enumGetDefaultEndpoint, flow, eConsole, uintptr(unsafe.Pointer(&dev))); failed(r) {
			return nil, hresultErr("GetDefaultAudioEndpoint", r)
		}
	} else {
		id, err := windows.UTF16PtrFromString(c.deviceID)
		if err != nil {
			return nil, err
		}
		if r := comCall(en, enumGetDevice, uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(&dev))); failed(r) {
			return nil, hresultErr("GetDevice", r)
		}
	}
	defer release(dev)

	s := &wasapiStream{}
	if r := comCall(dev, devActivate, uintptr(unsafe.Pointer(&iidIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&s.client))); failed(r) {
		return nil, hresultErr("Activate(IAudioClient)", r)
	}
	wfx := waveFormatExtensible{
		FormatTag: 0xFFFE, Channels: audioChannels, SamplesPerSec: audioRate,
		AvgBytesPerSec: audioRate * audioChannels * 4, BlockAlign: audioChannels * 4, BitsPerSample: 32,
		CbSize: 22, ValidBitsPerSample: 32, ChannelMask: 0x3, SubFormat: ksSubtypeIEEEFloat,
	}
	flags := uintptr(audclntAutoConvertPCM | audclntSrcDefaultQualty)
	if c.loopback {
		flags |= audclntLoopback
	}
	if r := comCall(s.client, acInitialize, audclntShareShared, flags, audioBuffer100ns, 0, uintptr(unsafe.Pointer(&wfx)), 0); failed(r) {
		s.close()
		if uint32(r) == 0x80070005 {
			return nil, errors.New("access denied: allow desktop apps to use the microphone in Windows privacy settings")
		}
		return nil, hresultErr("IAudioClient.Initialize", r)
	}
	if r := comCall(s.client, acGetService, uintptr(unsafe.Pointer(&iidIAudioCaptureClient)), uintptr(unsafe.Pointer(&s.capture))); failed(r) {
		s.close()
		return nil, hresultErr("GetService(IAudioCaptureClient)", r)
	}
	if r := comCall(s.client, acStart); failed(r) {
		s.close()
		return nil, hresultErr("IAudioClient.Start", r)
	}
	return s, nil
}

// startWASAPI opens the device synchronously (so a missing permission or
// device is reported by Start) and then captures on its own thread.
func startWASAPI(src *audioSource, loopback bool, deviceID string, origin int64) (*wasapiCapture, error) {
	c := &wasapiCapture{src: src, loopback: loopback, deviceID: deviceID, origin: origin, stop: make(chan struct{})}
	ready := make(chan error, 1)
	c.wg.Add(1)
	go c.run(ready)
	if err := <-ready; err != nil {
		c.wg.Wait()
		return nil, err
	}
	return c, nil
}

func (c *wasapiCapture) run(ready chan<- error) {
	defer c.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procCoInitializeEx.Call(0, coinitMultithreaded)

	s, err := c.open()
	ready <- err
	if err != nil {
		return
	}
	defer func() { s.close() }()

	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-tick.C:
		}
		if err := c.drain(s); err != nil {
			if uint32(hresultOf(err)) != audclntDeviceInvalid {
				c.err = err
				return
			}
			// Default device changed or the device was unplugged: reopen
			// and keep going; the mixer fills the hole with silence.
			s.close()
			for {
				select {
				case <-c.stop:
					s = &wasapiStream{}
					return
				case <-time.After(250 * time.Millisecond):
				}
				if s, err = c.open(); err == nil {
					c.reopens++
					break
				}
			}
		}
	}
}

type hresultError uintptr

func (h hresultError) Error() string { return hresultErr("WASAPI", uintptr(h)).Error() }

func hresultOf(err error) uintptr {
	var h hresultError
	if errors.As(err, &h) {
		return uintptr(h)
	}
	return 0
}

func (c *wasapiCapture) drain(s *wasapiStream) error {
	for {
		var next uint32
		if r := comCall(s.capture, accGetNextPacketSize, uintptr(unsafe.Pointer(&next))); failed(r) {
			return hresultError(r)
		}
		if next == 0 {
			return nil
		}
		var (
			data          *float32
			frames, flags uint32
			devPos, qpc   uint64
		)
		if r := comCall(s.capture, accGetBuffer, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&frames)),
			uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&devPos)), uintptr(unsafe.Pointer(&qpc))); failed(r) {
			return hresultError(r)
		}
		pos := (int64(qpc) - c.origin) * audioRate / hundredNsPerSec
		if flags&audclntBufferSilent != 0 || data == nil {
			c.src.push(pos, nil, int64(frames))
		} else {
			pcm := unsafe.Slice(data, int(frames)*audioChannels)
			c.src.push(pos, append([]float32(nil), pcm...), int64(frames))
		}
		comCall(s.capture, accReleaseBuffer, uintptr(frames))
	}
}

func (c *wasapiCapture) close() {
	close(c.stop)
	c.wg.Wait()
	c.src.mu.Lock()
	c.src.stats.Reopens = c.reopens
	if c.err != nil {
		c.src.stats.Err = c.err.Error()
	}
	c.src.mu.Unlock()
}
