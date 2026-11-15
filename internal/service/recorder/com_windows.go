//go:build windows

package recorder

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Minimal COM / WinRT plumbing: vtable calls, GUIDs and HSTRINGs. Vtable
// indices are taken from the Windows SDK headers (counting IUnknown's three
// methods, plus IInspectable's three for WinRT interfaces).

var (
	combase     = windows.NewLazySystemDLL("combase.dll")
	d3d11dll    = windows.NewLazySystemDLL("d3d11.dll")
	mfplat      = windows.NewLazySystemDLL("mfplat.dll")
	mfreadwrite = windows.NewLazySystemDLL("mfreadwrite.dll")
	user32      = windows.NewLazySystemDLL("user32.dll")
	dwmapi      = windows.NewLazySystemDLL("dwmapi.dll")

	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")

	procD3D11CreateDevice                    = d3d11dll.NewProc("D3D11CreateDevice")
	procCreateDirect3D11DeviceFromDXGIDevice = d3d11dll.NewProc("CreateDirect3D11DeviceFromDXGIDevice")

	procMFStartup                 = mfplat.NewProc("MFStartup")
	procMFShutdown                = mfplat.NewProc("MFShutdown")
	procMFCreateAttributes        = mfplat.NewProc("MFCreateAttributes")
	procMFCreateMediaType         = mfplat.NewProc("MFCreateMediaType")
	procMFCreateSample            = mfplat.NewProc("MFCreateSample")
	procMFCreateMemoryBuffer      = mfplat.NewProc("MFCreateMemoryBuffer")
	procMFCreateDXGIDeviceManager = mfplat.NewProc("MFCreateDXGIDeviceManager")
	procMFCreateDXGISurfaceBuffer = mfplat.NewProc("MFCreateDXGISurfaceBuffer")
	procMFCreateSinkWriterFromURL = mfreadwrite.NewProc("MFCreateSinkWriterFromURL")

	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	procMonitorFromRect          = user32.NewProc("MonitorFromRect")
	procGetMonitorInfoW          = user32.NewProc("GetMonitorInfoW")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procDwmGetWindowAttribute    = dwmapi.NewProc("DwmGetWindowAttribute")
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var (
	iidIGraphicsCaptureItemInterop        = mustGUID("3628E81B-3CAC-4C60-B7F4-23CE0E0C3356")
	iidIGraphicsCaptureItem               = mustGUID("79C3F95B-31F7-4EC2-A464-632EF5D30760")
	iidIDirect3D11CaptureFramePoolStatic2 = mustGUID("589B103F-6BBC-5DF5-A991-02E28B3B66D5")
	iidIGraphicsCaptureSession2           = mustGUID("2C39AE40-7D2E-5044-804E-8B6799D4CF9E")
	iidIGraphicsCaptureSession3           = mustGUID("F2CDD966-22AE-5EA1-9596-3A289344C3BE")
	iidIDirect3DDxgiInterfaceAccess       = mustGUID("A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1")
	iidIDirect3DDevice                    = mustGUID("A37624AB-8D5F-4650-9D3E-9EAE3D9BC670")
	iidIClosable                          = mustGUID("30D5A829-7FA4-4026-83BB-D75BAE4EA99E")

	iidIDXGIDevice       = mustGUID("54EC77FA-1377-44E6-8C32-88FD5F44C84C")
	iidID3D11Texture2D   = mustGUID("6F15AAF2-D208-4E89-9AB4-489535D34F9C")
	iidID3D10Multithread = mustGUID("9B7E4E00-342C-4106-A19F-4F2704F689F0")
	iidIMF2DBuffer       = mustGUID("7DC9D5F9-9ED9-44EC-9BBF-0600BB589FBB")
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

// closeWinRT calls IClosable.Close, which WinRT capture objects need to stop
// promptly instead of waiting for the last reference to go away.
func closeWinRT(obj unsafe.Pointer) {
	if obj == nil {
		return
	}
	if c, err := queryInterface(obj, &iidIClosable); err == nil {
		comCall(c, 6)
		release(c)
	}
}

// activationFactory returns the WinRT activation factory of a runtime class.
func activationFactory(class string, iid *windows.GUID) (unsafe.Pointer, error) {
	name, err := windows.UTF16FromString(class)
	if err != nil {
		return nil, err
	}
	var hs uintptr
	if r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)-1), uintptr(unsafe.Pointer(&hs))); failed(r) {
		return nil, hresultErr("WindowsCreateString", r)
	}
	defer procWindowsDeleteString.Call(hs)

	var f unsafe.Pointer
	if r, _, _ := procRoGetActivationFactory.Call(hs, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f))); failed(r) {
		return nil, hresultErr("RoGetActivationFactory("+class+")", r)
	}
	return f, nil
}
