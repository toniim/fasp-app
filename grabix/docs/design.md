# Cross‑platform Screenshot & Annotation App

## 1. Product Scope & Core Features
The application is a lightweight desktop screenshot and annotation tool supporting **macOS and Windows**.

Core features:
- Capture fullscreen / active display
- Freeze screen and select a region
- Annotate: highlight, rectangle, arrow, text
- Save locally or upload (cloud / API)
- Global hotkeys and tray integration

Non‑goals (v1):
- Video capture / GIF recording
- Advanced image editing (blur brush, layers, filters)

---

## 2. High‑level Architecture

**Framework choice**: Wails (Go backend + Web frontend)

Rationale:
- Native access for screenshot, hotkeys, filesystem
- Web stack for fast UI iteration and cross‑platform rendering
- Minimal per‑OS native code

Architecture overview:

- Go Backend: OS integration, capture, IO, upload
- Frontend (React + Canvas): region select, editor, UX

---

## 3. Backend Design (Go)

### 3.1 Core Services

**CaptureService**
- CaptureFullscreen()
- CaptureActiveDisplay()
- CaptureAllDisplays()

Returns raw image bytes (PNG) or base64 for frontend rendering.

**FileService**
- OpenSaveDialog()
- SaveImage(path, format, quality, bytes)
- Auto‑naming (timestamp, display index)

**UploadService**
- Upload(imageBytes) → { url, id }
- Pluggable providers via interface

**HotkeyService**
- Global hotkey registration
- Triggers capture flow
- OS‑specific implementation via build tags

**SettingsService**
- Hotkeys
- Default save path
- Upload provider config

---

### 3.2 OS‑specific Boundaries

Isolated per‑OS code using Go build tags:

- hotkeys_windows.go
- hotkeys_darwin.go

Screenshot capture relies on cross‑platform library where possible.

macOS‑specific handling:
- Screen Recording permission detection
- User guidance if permission missing

---

## 4. Frontend Design (React + TypeScript)

### 4.1 Windows

**CaptureWindow** (Fullscreen, borderless)
- Displays frozen screenshot
- Crosshair cursor
- Drag to select region
- Emits crop rectangle

**EditorWindow**
- Canvas‑based image editor
- Tools:
  - Rectangle / box
  - Arrow
  - Highlight (semi‑transparent)
  - Text
- Undo / redo
- Export image

---

### 4.2 Canvas & Editor Technology

Recommended library: **Konva.js**

Reasons:
- Scene graph abstraction
- Built‑in transforms (resize, rotate)
- Simple undo via JSON snapshots
- Good text and arrow support

Alternative:
- Fabric.js (acceptable fallback)

---

## 5. Capture & Edit Flow

### 5.1 Fullscreen Capture
1. Hotkey pressed
2. Go captures active display
3. Frontend opens EditorWindow with image

### 5.2 Freeze & Region Select
1. Hotkey pressed
2. Go captures active display
3. CaptureWindow opens fullscreen
4. User selects region
5. Frontend crops image
6. EditorWindow opens with cropped result

Client‑side cropping is preferred for speed and UX.

---

## 6. Save & Upload Flow

### Save
- Frontend exports image
- Calls FileService.SaveImage
- Supports PNG / JPEG / WebP

### Upload
- Frontend exports image
- Calls UploadService.Upload
- Receives URL
- Copies URL to clipboard

Upload providers are swappable.

---

## 7. Cross‑platform Considerations

### macOS
- Screen Recording permission (TCC)
- Code signing & notarization for distribution

### Windows
- DPI scaling and multi‑monitor coordinate normalization
- Always‑on‑top fullscreen window

---

## 8. Suggested Project Structure

```
/internal
  /capture
  /imageops
  /hotkeys
  /storage
  /upload
  /settings
/frontend
  /capture
  /editor
  /services
```

---

## 9. Implementation Roadmap

Phase 1 (MVP)
- Screenshot capture
- Freeze + region select
- Basic annotation
- Save to disk

Phase 2
- Upload providers
- Hotkey customization
- Tray menu

Phase 3
- Polishing, performance tuning
- Auto‑update, signing, packaging

---

## 10. Design Principles

- Frontend handles all visual interaction
- Backend handles OS boundaries only
- Avoid native overlays when web UI can suffice
- Keep OS‑specific code isolated

