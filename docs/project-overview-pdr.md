# Grabix - Project Overview & Product Development Requirements

## 1. Vision & Product Statement

Grabix is a **lightweight, cross-platform desktop screenshot and annotation tool** that enables users to capture, annotate, and share screenshots in seconds. Built with native performance (Go backend) and modern UI (React frontend), Grabix bridges the gap between simple screenshot utilities and feature-heavy image editors.

**Mission**: Streamline visual communication by providing the fastest, most intuitive annotation experience on macOS and Windows.

---

## 2. Core Features (MVP Complete)

### Capture
- **Fullscreen Capture**: One-click screenshot of entire display
- **Active Display**: Capture the currently active monitor (multi-display support)
- **Region Selection**: Freeze screen, drag to select area with visual feedback

### Annotation & Editing
- **Shape Tools**: Rectangle, arrow, highlight (semi-transparent overlay)
- **Text Tool**: Add notes with customizable font and color
- **Colors & Transparency**: Adjust stroke color, fill color, opacity per annotation
- **Undo/Redo**: Full history support with Ctrl+Z / Ctrl+Y
- **Zoom & Pan**: Navigate large screenshots with mouse wheel and drag

### Save & Share
- **Save to Disk**: PNG (lossless), JPEG (compact), WebP (modern)
- **Copy to Clipboard**: Instant sharing via paste (macOS + Windows)
- **Auto-naming**: Timestamps and display info in filenames
- **Custom Paths**: Choose default save location in settings

### System Integration
- **Global Hotkeys**: Customizable keyboard shortcuts (default: Ctrl+Shift+S / Cmd+Shift+S)
- **System Tray**: Quick access menu bar (macOS) or system tray (Windows)
- **Run at Startup**: Optional auto-launch on login (macOS)
- **Permission Handling**: Smart dialogs for Screen Recording (macOS), Accessibility (Windows)

### Settings & Preferences
- **Hotkey Configuration**: Rebind capture hotkeys to user preference
- **Format Selection**: Default save format (PNG/JPEG/WebP)
- **Upload Providers**: Extensible system for future cloud integrations
- **Persistent Config**: JSON-based settings stored locally (`~/.grabix/settings.json`)

---

## 3. Target Platforms & Audience

**Supported Platforms**:
- macOS 11+ (Intel and Apple Silicon)
- Windows 10+ (build 1909+)

**Target Users**:
- Content creators (tutorial makers, designers, developers)
- Product managers documenting feedback
- QA engineers creating bug reports
- Technical writers needing quick screenshots
- Developers collaborating via visual communication

**Out-of-Scope (Phase 1)**:
- Video/GIF recording
- Advanced image editing (filters, layers, brush tools)
- OCR or text detection
- Collaboration/sharing to cloud services (future Phase 2)
- Mobile platforms

---

## 4. Functional Requirements (FR)

| ID | Requirement | Priority | Status |
|----|------------|----------|--------|
| FR-1 | Capture active display or fullscreen | P0 | Done |
| FR-2 | Display frozen screenshot, allow region selection via drag | P0 | Done |
| FR-3 | Annotate with rectangle, arrow, text, highlight | P0 | Done |
| FR-4 | Support undo/redo for all annotations | P0 | Done |
| FR-5 | Save image as PNG/JPEG/WebP to disk | P0 | Done |
| FR-6 | Copy annotated image to clipboard | P0 | Done |
| FR-7 | Register global hotkeys (macOS/Windows) | P0 | Done |
| FR-8 | System tray with capture menu (macOS) | P0 | Done |
| FR-9 | Load/save settings (JSON config) | P1 | Done |
| FR-10 | Handle macOS Screen Recording permissions | P1 | Done |
| FR-11 | Multi-display coordinate normalization (Windows) | P1 | Done |
| FR-12 | Run at startup option (macOS) | P1 | Done |
| FR-13 | Customizable hotkey bindings | P2 | Not started |
| FR-14 | Upload to configurable providers (HTTP) | P2 | Not started |
| FR-15 | Settings UI window | P2 | Partial |

---

## 5. Non-Functional Requirements (NFR)

| Requirement | Target | Status |
|-------------|--------|--------|
| Capture latency | <500ms | Achieved |
| Annotation rendering | 60 FPS smooth | Achieved |
| Startup time | <2s | Achieved |
| Binary size | <25MB | ~15MB macOS |
| Memory usage | <50MB baseline | Achieved |
| Supported languages | English (v1) | Current |

---

## 6. Technical Constraints & Decisions

### Architecture
- **Framework**: Wails v2.11 (Go backend + React frontend)
- **Backend**: Go 1.24, service-oriented design with interfaces
- **Frontend**: React 18 + TypeScript, Konva.js for canvas editing
- **State Management**: Zustand for React store (lightweight, no Context hell)
- **IPC Communication**: Wails runtime for Go ↔ frontend messaging

### Platform Abstraction
- Go build tags (`_darwin.go`, `_windows.go`) isolate OS-specific code
- Minimal native code (primarily macOS permissions and tray integration)
- Cross-platform libraries where possible (golang.design/x/hotkey)

### Key Rationale
- **Wails over Electron**: 10MB binary vs 100MB+, native performance with web UI flexibility
- **Konva.js over Fabric.js**: Scene-graph abstraction, built-in transforms, simpler undo/redo
- **Zustand over Redux**: Minimal boilerplate, no Context overhead, perfect for this scale
- **Client-side image cropping**: Faster UX, reduces backend complexity

---

## 7. Success Metrics (Phase 1 MVP)

### Performance
- ✅ Capture screenshot: <500ms
- ✅ Smooth annotation rendering: 60 FPS
- ✅ Save image: <1s
- ✅ Startup time: <2s
- ✅ App size: <25MB

### Reliability
- ✅ No crashes during 1-hour stress test
- ✅ All manual test cases pass on macOS + Windows
- ✅ Permission handling graceful on both platforms

### Code Quality
- ✅ Test coverage: ≥50% (Go backend)
- ✅ No unhandled errors in happy path
- ✅ Comprehensive error messages for users

---

## 8. Development Phases

### Phase 1: MVP (COMPLETE)
**Duration**: ~4 weeks
**Status**: Done (screenshot, annotate, save, hotkeys, tray)
**Deliverables**:
- Core capture and annotation engine
- Hotkey integration
- System tray (macOS)
- Settings persistence

### Phase 2: Advanced Features (NEXT)
**Duration**: ~2 weeks
**Goals**:
- Hotkey customization UI
- Upload provider system (HTTP, clipboard)
- Multi-monitor fixes (Windows)
- Settings window polish
- Keyboard shortcuts in editor (Ctrl+Z, Ctrl+S)

### Phase 3: Polish & Distribution (FUTURE)
**Duration**: ~3 weeks
**Goals**:
- Code signing + notarization (macOS)
- Installer creation (.dmg, .exe)
- Auto-update system
- Comprehensive user documentation
- Release CI/CD (GitHub Actions)
- Community feedback integration

---

## 9. Acceptance Criteria (Phase 1 MVP)

1. **Capture & Edit Flow**
   - User presses hotkey (Cmd+Shift+S / Ctrl+Shift+S)
   - Frozen screenshot appears fullscreen with crosshair
   - Drag to select region → crop automatically
   - Editor window opens with image ready for annotation
   - User can add shapes, text, undo changes

2. **Save & Share**
   - User clicks "Save" → native file dialog appears
   - File saved to disk with timestamp filename
   - User clicks "Copy" → PNG copied to clipboard for paste elsewhere
   - Verify save works with PNG, JPEG, WebP formats

3. **System Integration**
   - Hotkey works from any app (global registration)
   - System tray shows on macOS (menu bar)
   - Settings persist after restart
   - macOS permissions dialog appears if needed (Screen Recording)

4. **Error Handling**
   - Capture denied → graceful error message
   - Save to locked location → user-friendly error
   - Hotkey conflict → warning with alternative
   - Invalid config → defaults to safe values

---

## 10. Risk Assessment

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|-----------|
| macOS permission denial | High | Medium | Clear user guidance, permission checks |
| Multi-display coordinate issues | Medium | Medium | Platform-specific testing early |
| Global hotkey conflicts | Medium | Low | Customizable bindings, conflict detection |
| Canvas performance (large images) | Medium | Low | Image resolution limits, optimization |
| Wails framework limitations | Medium | Low | Prototype early, fallback to native code |

---

## 11. Dependencies & Integrations

### External Libraries
- `github.com/wailsapp/wails/v2` - Framework (v2.11)
- `golang.design/x/hotkey` - Global hotkey registration
- `konva` + `react-konva` - Canvas rendering
- `zustand` - State management

### OS APIs Used
- **macOS**: CGDisplayCreateImage (screenshot), CGEventTap (hotkeys), NSStatusItem (tray)
- **Windows**: Win32 API (screenshot, DPI scaling, hotkey registration)

### Data Formats
- PNG/JPEG/WebP (image export)
- JSON (settings storage)
- Base64 (screenshot transmission between Go and frontend)

---

## 12. Configuration & Settings

### Default Settings
```json
{
  "hotkey": "cmd+shift+s",      // macOS default
  "format": "png",               // PNG, JPEG, WebP
  "quality": 95,                 // For JPEG/WebP
  "savePath": "~/Pictures",      // Default location
  "startupEnabled": false,       // Run at login
  "fontSize": 16,                // Annotation text size
  "strokeWidth": 2,              // Annotation stroke
  "colors": {
    "rect": "#FF0000",
    "arrow": "#00FF00",
    "text": "#000000"
  }
}
```

### Environment
- Config location: `~/.grabix/settings.json` (portable)
- Logs location: `~/.grabix/logs/` (future)
- Cache: None (stateless design)

---

## 13. Future Roadmap (Phase 2+)

- **Upload Providers**: Imgur, AWS S3, Dropbox, custom HTTP endpoint
- **Keyboard Shortcuts**: In-editor shortcuts (Ctrl+A, Ctrl+X, etc.)
- **Color Picker**: UI for custom annotation colors
- **Template/Presets**: Save annotation styles for reuse
- **Clipboard Monitoring**: Auto-launch when image pasted
- **Batch Export**: Watermarking, resizing, format conversion
- **Collaboration**: Share links with expiry, commenting
- **macOS App Clips**: Quick capture from Control Center (iOS 17+)
- **Windows Toast Notifications**: Save confirmation alerts

---

## 14. Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-02-01 | Team | Initial PDR, MVP complete |

---

**Status**: MVP Complete, Ready for Phase 2
**Next Review**: After Phase 2 completion
