//go:build windows

package recorder

import (
	"errors"
	"unsafe"
)

const (
	d3dDriverTypeHardware      = 1
	d3dCreateDeviceBGRASupport = 0x20
	d3dCreateDeviceVideo       = 0x800
	d3dSDKVersion              = 7

	dxgiFormatB8G8R8A8 = 87

	d3dUsageDefault     = 0
	d3dUsageStaging     = 3
	d3dBindShaderRes    = 0x8
	d3dBindRenderTarget = 0x20
	d3dCPUAccessRead    = 0x20000
	d3dMapRead          = 1

	// ID3D11Device / ID3D11DeviceContext / ID3D11Texture2D vtable indices.
	devCreateTexture2D        = 5
	ctxMap                    = 14
	ctxUnmap                  = 15
	ctxCopySubresourceRegion  = 46
	ctxCopyResource           = 47
	texGetDesc                = 10
	mtSetMultithreadProtected = 5
)

type texture2DDesc struct {
	Width, Height, MipLevels, ArraySize, Format uint32
	SampleCount, SampleQuality                  uint32
	Usage, BindFlags, CPUAccessFlags, MiscFlags uint32
}

type d3dBox struct{ Left, Top, Front, Right, Bottom, Back uint32 }

type mappedSubresource struct {
	Data       unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

// d3dDevice is the hardware D3D11 device shared by WGC (which renders the
// captured frames into it) and Media Foundation (which encodes from it).
type d3dDevice struct {
	dev, ctx unsafe.Pointer
	winrt    unsafe.Pointer // IDirect3DDevice wrapping dev, for WGC
	video    bool           // created with VIDEO_SUPPORT: usable by MF's GPU path
}

func newD3DDevice() (*d3dDevice, error) {
	d := &d3dDevice{video: true}
	create := func(flags uintptr) uintptr {
		r, _, _ := procD3D11CreateDevice.Call(0, d3dDriverTypeHardware, 0, flags, 0, 0, d3dSDKVersion,
			uintptr(unsafe.Pointer(&d.dev)), 0, uintptr(unsafe.Pointer(&d.ctx)))
		return r
	}
	r := create(d3dCreateDeviceBGRASupport | d3dCreateDeviceVideo)
	if failed(r) {
		d.video = false
		if r = create(d3dCreateDeviceBGRASupport); failed(r) {
			return nil, hresultErr("D3D11CreateDevice", r)
		}
	}

	// Media Foundation uses the device from its own worker threads.
	if mt, err := queryInterface(d.dev, &iidID3D10Multithread); err == nil {
		comCall(mt, mtSetMultithreadProtected, 1)
		release(mt)
	}

	dxgi, err := queryInterface(d.dev, &iidIDXGIDevice)
	if err != nil {
		d.close()
		return nil, err
	}
	defer release(dxgi)
	var insp unsafe.Pointer
	if r, _, _ := procCreateDirect3D11DeviceFromDXGIDevice.Call(uintptr(dxgi), uintptr(unsafe.Pointer(&insp))); failed(r) {
		d.close()
		return nil, hresultErr("CreateDirect3D11DeviceFromDXGIDevice", r)
	}
	defer release(insp)
	if d.winrt, err = queryInterface(insp, &iidIDirect3DDevice); err != nil {
		d.close()
		return nil, err
	}
	return d, nil
}

func (d *d3dDevice) close() {
	release(d.winrt)
	release(d.ctx)
	release(d.dev)
	*d = d3dDevice{}
}

// newTexture creates a BGRA texture: a GPU one (staging=false) for copying
// and encoding, or a CPU-readable staging one.
func (d *d3dDevice) newTexture(w, h int, staging bool) (unsafe.Pointer, error) {
	desc := texture2DDesc{
		Width: uint32(w), Height: uint32(h), MipLevels: 1, ArraySize: 1,
		Format: dxgiFormatB8G8R8A8, SampleCount: 1,
		Usage: d3dUsageDefault, BindFlags: d3dBindRenderTarget | d3dBindShaderRes,
	}
	if staging {
		desc.Usage, desc.BindFlags, desc.CPUAccessFlags = d3dUsageStaging, 0, d3dCPUAccessRead
	}
	var tex unsafe.Pointer
	if r := comCall(d.dev, devCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&tex))); failed(r) {
		return nil, hresultErr("CreateTexture2D", r)
	}
	return tex, nil
}

// copyRegion copies box of src to the top-left of dst.
func (d *d3dDevice) copyRegion(dst, src unsafe.Pointer, box d3dBox) {
	comCall(d.ctx, ctxCopySubresourceRegion, uintptr(dst), 0, 0, 0, 0, uintptr(src), 0, uintptr(unsafe.Pointer(&box)))
}

func (d *d3dDevice) copy(dst, src unsafe.Pointer) {
	comCall(d.ctx, ctxCopyResource, uintptr(dst), uintptr(src))
}

// readStaging maps a staging texture and hands its rows to fn.
func (d *d3dDevice) readStaging(tex unsafe.Pointer, fn func(data unsafe.Pointer, pitch int) error) error {
	var m mappedSubresource
	if r := comCall(d.ctx, ctxMap, uintptr(tex), 0, d3dMapRead, 0, uintptr(unsafe.Pointer(&m))); failed(r) {
		return hresultErr("Map", r)
	}
	defer comCall(d.ctx, ctxUnmap, uintptr(tex), 0)
	if m.Data == nil {
		return errors.New("Map returned no data")
	}
	return fn(m.Data, int(m.RowPitch))
}

func textureSize(tex unsafe.Pointer) (int, int) {
	var desc texture2DDesc
	comCall(tex, texGetDesc, uintptr(unsafe.Pointer(&desc)))
	return int(desc.Width), int(desc.Height)
}
