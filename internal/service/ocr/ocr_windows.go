//go:build windows

package ocr

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WinRT plumbing for Windows.Media.Ocr. Vtable indices count IUnknown's three
// methods plus IInspectable's three, so a WinRT interface's own methods start
// at 6. The index of every method below was checked against the Windows
// metadata (windows-rs generated bindings).

var (
	combase = windows.NewLazySystemDLL("combase.dll")

	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringBuffer = combase.NewProc("WindowsGetStringRawBuffer")
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var (
	iidIOcrEngineStatics      = mustGUID("5BFFA85A-3384-3540-9940-699120D428A8")
	iidIOcrResult             = mustGUID("9BD235B2-175B-3D6A-92E2-388C206E2F63")
	iidIOcrLine               = mustGUID("0043A16F-E31F-3A24-899C-D444BD088124")
	iidIOcrWord               = mustGUID("3C2A477A-5CD9-3525-BA2A-23D1E0A68A1D")
	iidISoftwareBitmapStatics = mustGUID("DF0385DB-672F-4A9D-806E-C2442F343E86")
	iidIBufferFactory         = mustGUID("71AF914D-C10F-484B-BC50-14BC623B3A27")
	iidIBuffer                = mustGUID("905A0FE0-BC53-11DF-8C49-001E4FC686DA")
	iidIBufferByteAccess      = mustGUID("905A0FEF-BC53-11DF-8C49-001E4FC686DA")
	iidILanguageFactory       = mustGUID("9B0252AC-0C27-44F8-B792-9793FB66C63E")
	iidILanguage              = mustGUID("EA79A752-F7C2-4265-B1BD-C4DEC4E4F080")
	iidIAsyncInfo             = mustGUID("00000036-0000-0000-C000-000000000046")
)

const (
	bitmapPixelFormatBgra8 = 87
	roInitMultithreaded    = 1

	asyncStarted   = 0
	asyncCompleted = 1

	// Upper bound for one OCR call, so a stuck engine cannot hang the app.
	asyncTimeout = 30 * time.Second
)

// comCall invokes method idx of the COM object obj.
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

// newHString creates an HSTRING; the caller deletes it with deleteHString.
func newHString(s string) (uintptr, error) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var hs uintptr
	if r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&hs))); failed(r) {
		return 0, hresultErr("WindowsCreateString", r)
	}
	return hs, nil
}

func deleteHString(hs uintptr) { procWindowsDeleteString.Call(hs) }

// takeHString converts an HSTRING returned by a WinRT getter to a Go string
// and frees it.
func takeHString(hs uintptr) string {
	if hs == 0 {
		return ""
	}
	defer deleteHString(hs)
	var n uint32
	p, _, _ := procWindowsGetStringBuffer.Call(hs, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	// p points into the HSTRING's own memory (not the Go heap), so reinterpret
	// the integer instead of converting it, which vet flags.
	buf := *(**uint16)(unsafe.Pointer(&p))
	return windows.UTF16ToString(unsafe.Slice(buf, int(n)))
}

func activationFactory(class string, iid *windows.GUID) (unsafe.Pointer, error) {
	hs, err := newHString(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(hs)
	var f unsafe.Pointer
	if r, _, _ := procRoGetActivationFactory.Call(hs, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f))); failed(r) {
		return nil, hresultErr("RoGetActivationFactory("+class+")", r)
	}
	return f, nil
}

// initWinRT pins the goroutine to its OS thread and joins the multithreaded
// apartment. The returned func undoes the pin. A thread that is already in a
// single-threaded apartment (RPC_E_CHANGED_MODE) still works for these calls.
func initWinRT() func() {
	runtime.LockOSThread()
	procRoInitialize.Call(roInitMultithreaded)
	return runtime.UnlockOSThread
}

// await polls an IAsyncOperation until it leaves the Started state. Polling
// avoids implementing a completion-handler COM object in Go.
func await(op unsafe.Pointer) error {
	info, err := queryInterface(op, &iidIAsyncInfo)
	if err != nil {
		return err
	}
	defer release(info)
	deadline := time.Now().Add(asyncTimeout)
	for {
		var status int32
		if r := comCall(info, 7, uintptr(unsafe.Pointer(&status))); failed(r) {
			return hresultErr("IAsyncInfo.Status", r)
		}
		switch status {
		case asyncStarted:
			if time.Now().After(deadline) {
				comCall(info, 9) // Cancel
				return errors.New("ocr timed out")
			}
			time.Sleep(5 * time.Millisecond)
		case asyncCompleted:
			return nil
		default:
			var code uintptr
			comCall(info, 8, uintptr(unsafe.Pointer(&code)))
			return hresultErr("ocr operation failed", code)
		}
	}
}

// newLanguage builds a Windows.Globalization.Language from a BCP 47 tag.
// ok is false when the tag is not valid.
func newLanguage(tag string) (lang unsafe.Pointer, ok bool) {
	f, err := activationFactory("Windows.Globalization.Language", &iidILanguageFactory)
	if err != nil {
		return nil, false
	}
	defer release(f)
	hs, err := newHString(tag)
	if err != nil {
		return nil, false
	}
	defer deleteHString(hs)
	if r := comCall(f, 6, hs, uintptr(unsafe.Pointer(&lang))); failed(r) {
		return nil, false
	}
	return lang, true
}

// createEngine picks the engine for the first requested language that is
// installed, else uses the user's profile languages.
func createEngine(statics unsafe.Pointer, languages []string) (unsafe.Pointer, error) {
	for _, tag := range languages {
		lang, ok := newLanguage(tag)
		if !ok {
			continue
		}
		var supported int32
		r := comCall(statics, 8, uintptr(lang), uintptr(unsafe.Pointer(&supported)))
		var engine unsafe.Pointer
		if !failed(r) && supported != 0 {
			comCall(statics, 9, uintptr(lang), uintptr(unsafe.Pointer(&engine)))
		}
		release(lang)
		if engine != nil {
			return engine, nil
		}
	}
	var engine unsafe.Pointer
	if r := comCall(statics, 10, uintptr(unsafe.Pointer(&engine))); failed(r) {
		return nil, hresultErr("TryCreateFromUserProfileLanguages", r)
	}
	if engine == nil {
		return nil, ErrNoLanguage
	}
	return engine, nil
}

// newSoftwareBitmap copies BGRA pixels into a SoftwareBitmap through a WinRT
// Buffer (IBufferByteAccess exposes its memory).
func newSoftwareBitmap(pix []byte, w, h int) (unsafe.Pointer, error) {
	bf, err := activationFactory("Windows.Storage.Streams.Buffer", &iidIBufferFactory)
	if err != nil {
		return nil, err
	}
	defer release(bf)
	var buf unsafe.Pointer
	if r := comCall(bf, 6, uintptr(len(pix)), uintptr(unsafe.Pointer(&buf))); failed(r) {
		return nil, hresultErr("Buffer.Create", r)
	}
	defer release(buf)

	access, err := queryInterface(buf, &iidIBufferByteAccess)
	if err != nil {
		return nil, err
	}
	defer release(access)
	var mem *byte
	if r := comCall(access, 3, uintptr(unsafe.Pointer(&mem))); failed(r) || mem == nil {
		return nil, hresultErr("IBufferByteAccess.Buffer", r)
	}
	copy(unsafe.Slice(mem, len(pix)), pix)
	if r := comCall(buf, 8, uintptr(len(pix))); failed(r) { // put_Length
		return nil, hresultErr("IBuffer.Length", r)
	}

	sbs, err := activationFactory("Windows.Graphics.Imaging.SoftwareBitmap", &iidISoftwareBitmapStatics)
	if err != nil {
		return nil, err
	}
	defer release(sbs)
	var bmp unsafe.Pointer
	if r := comCall(sbs, 9, uintptr(buf), bitmapPixelFormatBgra8, uintptr(w), uintptr(h), uintptr(unsafe.Pointer(&bmp))); failed(r) {
		return nil, hresultErr("SoftwareBitmap.CreateCopyFromBuffer", r)
	}
	return bmp, nil
}

// Recognize runs Windows.Media.Ocr on a PNG. languages are BCP 47 tags; the
// first one with an installed OCR pack is used, else the user's profile
// languages.
func Recognize(png []byte, languages []string) (*Result, error) {
	if len(png) == 0 {
		return nil, errors.New("ocr: empty image")
	}
	pix, w, h, err := toBGRA(png)
	if err != nil {
		return nil, fmt.Errorf("ocr: decode image: %w", err)
	}

	defer initWinRT()()

	statics, err := activationFactory("Windows.Media.Ocr.OcrEngine", &iidIOcrEngineStatics)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	defer release(statics)

	var maxDim uint32
	if r := comCall(statics, 6, uintptr(unsafe.Pointer(&maxDim))); !failed(r) && maxDim > 0 && (uint32(w) > maxDim || uint32(h) > maxDim) {
		return nil, fmt.Errorf("ocr: image %dx%d exceeds the engine limit of %d pixels", w, h, maxDim)
	}

	engine, err := createEngine(statics, languages)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	defer release(engine)

	bmp, err := newSoftwareBitmap(pix, w, h)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	defer release(bmp)

	var op unsafe.Pointer
	if r := comCall(engine, 6, uintptr(bmp), uintptr(unsafe.Pointer(&op))); failed(r) { // RecognizeAsync
		return nil, fmt.Errorf("ocr: %w", hresultErr("RecognizeAsync", r))
	}
	defer release(op)
	if err := await(op); err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	var result unsafe.Pointer
	if r := comCall(op, 8, uintptr(unsafe.Pointer(&result))); failed(r) { // GetResults
		return nil, fmt.Errorf("ocr: %w", hresultErr("GetResults", r))
	}
	defer release(result)

	lines, err := readLines(result)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	return buildResult(lines), nil
}

// vectorSize and vectorAt read an IVectorView<T> (Size at 7, GetAt at 6).
func vectorSize(v unsafe.Pointer) (uint32, error) {
	var n uint32
	if r := comCall(v, 7, uintptr(unsafe.Pointer(&n))); failed(r) {
		return 0, hresultErr("IVectorView.Size", r)
	}
	return n, nil
}

func vectorAt(v unsafe.Pointer, i uint32) (unsafe.Pointer, error) {
	var item unsafe.Pointer
	if r := comCall(v, 6, uintptr(i), uintptr(unsafe.Pointer(&item))); failed(r) {
		return nil, hresultErr("IVectorView.GetAt", r)
	}
	return item, nil
}

// readLines converts an OcrResult into Lines. OcrLine has no box of its own,
// so each line's box is the union of its words' boxes.
func readLines(result unsafe.Pointer) ([]Line, error) {
	res, err := queryInterface(result, &iidIOcrResult)
	if err != nil {
		return nil, err
	}
	defer release(res)
	var vec unsafe.Pointer
	if r := comCall(res, 6, uintptr(unsafe.Pointer(&vec))); failed(r) { // Lines
		return nil, hresultErr("OcrResult.Lines", r)
	}
	defer release(vec)
	n, err := vectorSize(vec)
	if err != nil {
		return nil, err
	}
	lines := make([]Line, 0, n)
	for i := uint32(0); i < n; i++ {
		item, err := vectorAt(vec, i)
		if err != nil {
			return nil, err
		}
		l, err := readLine(item)
		release(item)
		if err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, nil
}

func readLine(item unsafe.Pointer) (Line, error) {
	line, err := queryInterface(item, &iidIOcrLine)
	if err != nil {
		return Line{}, err
	}
	defer release(line)

	var hs uintptr
	if r := comCall(line, 7, uintptr(unsafe.Pointer(&hs))); failed(r) { // Text
		return Line{}, hresultErr("OcrLine.Text", r)
	}
	out := Line{Text: takeHString(hs)}

	var words unsafe.Pointer
	if r := comCall(line, 6, uintptr(unsafe.Pointer(&words))); failed(r) { // Words
		return Line{}, hresultErr("OcrLine.Words", r)
	}
	defer release(words)
	n, err := vectorSize(words)
	if err != nil {
		return Line{}, err
	}
	var minX, minY, maxX, maxY float32
	for i := uint32(0); i < n; i++ {
		wi, err := vectorAt(words, i)
		if err != nil {
			return Line{}, err
		}
		word, err := queryInterface(wi, &iidIOcrWord)
		release(wi)
		if err != nil {
			return Line{}, err
		}
		var rect struct{ X, Y, W, H float32 }
		r := comCall(word, 6, uintptr(unsafe.Pointer(&rect))) // BoundingRect
		release(word)
		if failed(r) {
			return Line{}, hresultErr("OcrWord.BoundingRect", r)
		}
		if i == 0 {
			minX, minY, maxX, maxY = rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H
			continue
		}
		minX, minY = min(minX, rect.X), min(minY, rect.Y)
		maxX, maxY = max(maxX, rect.X+rect.W), max(maxY, rect.Y+rect.H)
	}
	out.X, out.Y, out.W, out.H = float64(minX), float64(minY), float64(maxX-minX), float64(maxY-minY)
	return out, nil
}

// SupportedLanguages lists the OCR languages installed on this machine.
func SupportedLanguages() ([]string, error) {
	defer initWinRT()()

	statics, err := activationFactory("Windows.Media.Ocr.OcrEngine", &iidIOcrEngineStatics)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	defer release(statics)

	var vec unsafe.Pointer
	if r := comCall(statics, 7, uintptr(unsafe.Pointer(&vec))); failed(r) { // AvailableRecognizerLanguages
		return nil, fmt.Errorf("ocr: %w", hresultErr("AvailableRecognizerLanguages", r))
	}
	defer release(vec)
	n, err := vectorSize(vec)
	if err != nil {
		return nil, fmt.Errorf("ocr: %w", err)
	}
	tags := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		item, err := vectorAt(vec, i)
		if err != nil {
			return nil, fmt.Errorf("ocr: %w", err)
		}
		lang, err := queryInterface(item, &iidILanguage)
		release(item)
		if err != nil {
			return nil, fmt.Errorf("ocr: %w", err)
		}
		var hs uintptr
		r := comCall(lang, 6, uintptr(unsafe.Pointer(&hs))) // LanguageTag
		release(lang)
		if failed(r) {
			return nil, fmt.Errorf("ocr: %w", hresultErr("Language.LanguageTag", r))
		}
		tags = append(tags, takeHString(hs))
	}
	return tags, nil
}
