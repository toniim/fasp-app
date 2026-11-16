# Screen recording and Capture Zone: design and spike results

**Updated**: 2026-11-15 · **Status**: Windows implemented and tested end to end. macOS has a recorder prototype that has never been compiled or run; Capture Zone and audio are not built on macOS yet (see [macOS work](#macos-work)).

## Scope

- **Screen recording** in three modes: **full screen**, **window** and **selected region**.
- **Audio**, chosen when a recording starts (defaults in Settings):
  - **computer sound**: the whole machine, not one app;
  - **microphone**: the user picks the device;
  - both are mixed into **one** AAC track;
  - echo cancellation is out of scope: most users wear headphones, and the dialog warns when speakers plus a microphone may echo.
- **Capture Zone**: a hotkey freezes the screen, the user drags a zone, and only that zone opens in the editor.
- Everything is native and in-process, for performance and stability: no ffmpeg, no helper process, no browser capture.

## Architecture

| | Windows | macOS (planned) |
|---|---|---|
| Video capture | **Windows.Graphics.Capture** (WGC) on a D3D11 device | **ScreenCaptureKit** `SCStream` |
| Video encode | **Media Foundation** sink writer → H.264 High (hardware MFT: NVENC / QuickSync / AMF, with a software fallback), GPU zero-copy | **AVAssetWriter** → VideoToolbox H.264 |
| Computer sound | **WASAPI loopback** on the default output device | `SCStreamConfiguration.capturesAudio` + `excludesCurrentProcessAudio` |
| Microphone | **WASAPI** capture, any device | AVCaptureSession (macOS 14); `captureMicrophone` on macOS 15 |
| Mixing | Shared Go mixer (`internal/service/recorder/mixer.go`) | same |
| Freeze overlay | Win32 top-most popup + GDI, in Go | borderless NSWindow at shielding level + NSView |
| Bindings | Pure Go COM/WinRT vtable calls, **no cgo** (like the GDI screenshot code) | cgo + Objective-C (like the ScreenCaptureKit screenshot code) |

Rejected alternatives:

- **DXGI Desktop Duplication**: cannot record a single window.
- **GDI per frame**: costs 30–60 ms per 1440p frame on the CPU.
- **Bundled ffmpeg**: about 80 MB, an extra process, and GPL/LGPL obligations.
- **WebView `getDisplayMedia`**: offers no control over its picker, records WebM, and is not native.
- **A WebView-based freeze overlay**: Wails cannot switch the window to frameless at runtime, it flickers while resizing, and pixels are not mapped 1:1 on HiDPI displays.

### Recording pipeline (Windows)

```
WGC frame pool ──► crop on GPU ──► "latest" texture ──► fixed-rate tick ──► MF sink writer ──► MP4
 (only changes)    (region mode)                         (repeats frame        (H.264 High)
                                                          when idle)
WASAPI loopback ─┐
                 ├─► 48 kHz stereo float ──► mixer: place packets by QPC time ──► PCM16 ──► AAC ─┘
WASAPI mic ──────┘   (AUTOCONVERTPCM)        fill silence, snap drift > 20 ms
```

- **One clock.** Video slots and audio packet times both come from QueryPerformanceCounter, and WASAPI reports QPC for every packet, so the two cannot drift apart.
- **Constant frame rate.** WGC delivers frames only when something changes; the loop writes the latest frame on every tick.
- **Silence.** Loopback delivers no packets while the computer is silent. The mixer fills the timeline with silence and lets late packets (up to 100 ms) catch up.
- **Device changes.** Plugging in headphones or unplugging the microphone ends the stream with `AUDCLNT_E_DEVICE_INVALIDATED`. The capture reopens the device; the gap is silent and the result panel shows a warning.
- **The window closes.** The recording stops by itself and keeps the file up to that moment.
- **The recording bar.** It is the Wails window, shrunk to an always-on-top bar. `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)` keeps it out of the video while it stays on screen.
- **Playing the result.** The recording opens in the editor's own player, as screenshots open in the editor, not in an external app:
  - **Serving the file**: the Wails asset server's `Handler` serves registered files only, under an unguessable `/media/<token>/<name>` URL. `http.ServeContent` answers Range requests, so seeking works without loading the whole file.
  - **Copy**: puts the file itself on the clipboard (`CF_HDROP` on Windows, a file URL on macOS).
  - **Save As**: copies the file to a location the user picks.
  - **Upload**: reuses the image upload flow with `video/mp4`.

### Capture Zone (Windows)

1. The monitor under the cursor is captured with GDI, as for normal screenshots.
2. A top-most popup covers that monitor and shows the frozen image dimmed, with crosshair guides.
3. The dragged area is shown undimmed, with a border and a `W × H` label.
4. The selected pixels are cropped from the frozen image, so the result is exactly what was frozen, then sent to the editor with the Select tool active.
5. Keys:
   - `Enter` takes the whole monitor;
   - `Esc` or a right-click cancels and brings back the editor if it was open;
   - a click without a drag is ignored.

The same overlay (`SelectRegion`) picks the area for region recordings.

## Results (Windows 11, 2560×1440 at 100% scaling)

### Recording performance

Measured with `cmd/recspike`, 5-second runs; each file was decoded with ffmpeg (PyAV) to check codec, frame count and timestamps.

| Run | Output | Frames written / dropped | CPU (one core) |
|---|---|---|---|
| Full screen, GPU zero-copy | 2560×1440 @ 30 | 150 / 1* | **13%** |
| Full screen, CPU copy path | 2560×1440 @ 30 | 150 / 1* | 52% |
| Full screen @ 60 fps | 2560×1440 @ 60 | 299 / 1* | 17% |
| Window (partly covered) | 1124×634 @ 30 | 149 / 1* | 14% |
| Region | 1280×720 @ 30 | 150 / 1* | 11% |

\* The last partial slot when recording stops, not a real drop.

### Audio and A/V sync

A test window flashed white and played a click at the same moment once a second. The recording was decoded and each flash compared with its click.

| Test | Result |
|---|---|
| 25 s, computer sound | audio – video offset **+18 ms** mean (σ 9 ms) |
| 10 min, computer sound + microphone, clicks in the first and last 30 s | offset **+6 ms at the start, +4 ms at the end**: no drift |
| Microphone over 10 min | 599.99 s received, **0 drift corrections** |
| CPU, 10 min with both audio sources | 10.8% of one core |

The offsets include `PlaySound`'s own output latency, so the recorder's own error is smaller.

### End-to-end tests in the app

Run against the real app: WebView2 driven over the DevTools protocol, with real global hotkeys and mouse drags from `SendInput`.

| Flow | Checked |
|---|---|
| `Shift+PrintScreen` → drag 600×400 | editor opens a 600×400 image at 100% with Select active; Fasp is hidden while the overlay is up |
| Capture Zone, then `Esc` | the editor comes back |
| Overlay drag / `Esc` / `Enter` / `SelectRegion` (harness) | 500×300 zone, cancel, whole monitor, correct desktop coordinates |
| `Ctrl+PrintScreen` → Window → target window closes | bar 300×110 at the top, top-most, affinity 0x11; recording stops by itself with a "window was closed" warning |
| Full screen + computer sound, stopped with the hotkey | 2560×1440 H.264 + AAC 48 kHz stereo; **the bar is absent from the video** |
| Region via the overlay, stopped with the bar's Stop button | 800×450 file; the window returns to its saved state, not top-most, affinity reset |
| Settings | three hotkey rows, recording defaults, real microphone list; saving re-registers hotkeys; a duplicate hotkey is refused |
| Player after stop | opens and plays at once (2560×1440, 4.1 s); seeking to 2.5 s works (Range); Space toggles playback; Esc closes; a new zone capture replaces it with the editor |
| Copy / Save As | the button and `Ctrl+C` put the file on the clipboard (read back as a file drop list); Save As through the native dialog writes an identical file, and cancelling does nothing |

Not covered on this machine:
- HiDPI scaling (the display is 100%), multiple monitors, and Windows 10 (its yellow capture border);
- uploading a video to a real fasp server (the server may not accept `video/mp4`);
- opening a recording from the tray **Recents**, which takes the same `open:video` path as a finished recording.

## Known limits and follow-ups

1. **Bitrate at 60 fps.** The encoder overshoots `MF_MT_AVG_BITRATE` by about 2×. Set the rate-control mode through `ICodecAPI`.
2. **Encoder width limit.** Hardware H.264 encoders stop at 4096 px wide. 5K and 6K displays need downscaling or HEVC.
3. **Texture allocation.** Each encoded frame allocates a GPU texture. Pool them (`IMFTrackedSample`) and run a long soak test for memory.
4. **Window resize during a recording.** Output stays at the starting size: the window's top-left part, padded with black. Consider scaling to fit.
5. **Multiple monitors.** A region is clamped to one monitor. Full screen records the monitor under the cursor.
6. **Clock drift.** The mixer corrects drift by snapping 20 ms steps. None were needed in the tests; adaptive resampling would remove even those.

## macOS work

To do on a Mac, in the same order as Windows:

1. **Compile and fix** `internal/service/recorder/recorder_darwin.{go,h,m}`. It is written against the ScreenCaptureKit/AVAssetWriter APIs but has never been built or run. Test all three modes on a Retina display.
2. **Audio.** System audio comes from `SCStreamOutputTypeAudio` on the same stream; the microphone from `AVCaptureSession`. Feed both into `audioSource.push`, using the host clock as the timeline, and add an AAC `AVAssetWriterInput`. Remove the "not implemented" guard in `Start`.
3. **Capture Zone and `SelectRegion`** in `internal/service/capture/capture_darwin.go`, currently stubs:
   - a borderless `NSWindow` at `CGShieldingWindowLevel` on the cursor's screen, with `canBecomeKeyWindow`;
   - a flipped `NSView` that draws the `SCScreenshotManager` image, dimmed;
   - mouse drag selects, Esc (key code 53) cancels;
   - crop with `CGImageCreateWithImageInRect`, scaled by the image size divided by the screen size.
4. **`Recording.Done()`** must also close when `SCStream` stops on its own (`didStopWithError`), so a closed window ends the recording.
5. **Recording bar.** It needs no affinity call: the content filter already excludes Fasp's windows. Check that the bar is excluded in Region mode too.
6. **Microphone permission.** `NSMicrophoneUsageDescription` is already in both Info.plist files. Check that the prompt appears, and whether the hardened runtime needs the `com.apple.security.device.audio-input` entitlement.
