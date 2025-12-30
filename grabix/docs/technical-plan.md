# Grabix - Technical Implementation Plan

## 1. Technology Stack

### Backend (Go)
- **Framework**: Wails v2 (latest stable)
- **Go Version**: 1.21+
- **Key Libraries**:
  - `github.com/kbinani/screenshot` - Cross-platform screenshot capture
  - `github.com/robotn/gohook` - Global hotkey registration
  - `github.com/skratchdot/open-golang/open` - Open files/URLs
  - Standard library: `image`, `image/png`, `encoding/base64`

### Frontend (React + TypeScript)
- **Framework**: React 18+ with TypeScript
- **Build Tool**: Vite (bundled with Wails)
- **Key Libraries**:
  - `konva` + `react-konva` - Canvas-based annotation editor
  - `zustand` - Lightweight state management
  - `react-hotkeys-hook` - Keyboard shortcuts in UI
  - `tailwindcss` - Styling

### Development Tools
- **Mockery**: Interface mocking for tests
- **Air**: Hot reload for Go development
- **ESLint + Prettier**: Code formatting
- **golangci-lint**: Go linting

---

## 2. Project Structure

```
grabix/
├── cmd/
│   └── grabix/
│       └── main.go                 # Application entry point
├── internal/
│   ├── app/
│   │   └── app.go                  # Wails app initialization
│   ├── service/
│   │   ├── capture/
│   │   │   ├── service.go          # CaptureService interface
│   │   │   ├── capture_darwin.go   # macOS implementation
│   │   │   └── capture_windows.go  # Windows implementation
│   │   ├── file/
│   │   │   └── service.go          # FileService (save, dialogs)
│   │   ├── upload/
│   │   │   ├── service.go          # UploadService interface
│   │   │   └── providers/          # Upload provider implementations
│   │   ├── hotkey/
│   │   │   ├── service.go          # HotkeyService interface
│   │   │   ├── hotkey_darwin.go    # macOS hotkeys
│   │   │   └── hotkey_windows.go   # Windows hotkeys
│   │   └── settings/
│   │       └── service.go          # SettingsService (config management)
│   ├── model/
│   │   └── types.go                # Shared data structures
│   └── util/
│       └── image.go                # Image processing utilities
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── CaptureWindow/      # Region selection UI
│   │   │   ├── EditorWindow/       # Annotation editor
│   │   │   └── TrayMenu/           # System tray menu
│   │   ├── hooks/                  # Custom React hooks
│   │   ├── store/                  # Zustand stores
│   │   ├── services/               # Wails bindings wrapper
│   │   ├── types/                  # TypeScript types
│   │   └── App.tsx                 # Main app component
│   ├── package.json
│   └── tsconfig.json
├── build/                          # Wails build config & assets
├── docs/
│   ├── design.md
│   └── technical-plan.md
├── scripts/
│   └── build.sh                    # Build automation
├── .mockery.yaml                   # Mockery configuration
├── wails.json                      # Wails project config
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## 3. Core Architecture

### 3.1 Backend Services (Go)

#### CaptureService Interface
```go
type CaptureService interface {
    CaptureFullscreen() ([]byte, error)
    CaptureActiveDisplay() ([]byte, error)
    CaptureAllDisplays() ([]DisplayCapture, error)
    GetDisplayInfo() ([]DisplayInfo, error)
}
```

#### FileService Interface
```go
type FileService interface {
    OpenSaveDialog(defaultName string) (string, error)
    SaveImage(path string, format string, quality int, data []byte) error
    GetDefaultSavePath() (string, error)
}
```

#### UploadService Interface
```go
type UploadService interface {
    Upload(data []byte, filename string) (*UploadResult, error)
    GetProviders() []string
    SetProvider(name string) error
}
```

#### HotkeyService Interface
```go
type HotkeyService interface {
    Register(key string, callback func()) error
    Unregister(key string) error
    IsRegistered(key string) bool
}
```

#### SettingsService Interface
```go
type SettingsService interface {
    Get(key string) (interface{}, error)
    Set(key string, value interface{}) error
    Load() error
    Save() error
}
```

### 3.2 Frontend Architecture

#### State Management (Zustand)
- **captureStore**: Current screenshot, crop region
- **editorStore**: Canvas state, annotations, undo/redo stack
- **settingsStore**: User preferences, hotkeys

#### Component Hierarchy
```
App
├── CaptureWindow (fullscreen overlay)
│   └── RegionSelector (drag selection)
├── EditorWindow (main editor)
│   ├── Toolbar (annotation tools)
│   ├── Canvas (Konva stage)
│   └── ActionBar (save, upload, copy)
└── TrayMenu (system tray integration)
```

---

## 4. Implementation Phases

### Phase 1: MVP (Week 1-2)
**Goal**: Basic screenshot + annotation + save

#### Tasks:
1. **Project Setup**
   - Initialize Wails project
   - Setup Go module structure
   - Configure frontend with React + TypeScript + Konva
   - Setup Makefile with common commands

2. **Backend - CaptureService**
   - Implement screenshot capture for macOS
   - Implement screenshot capture for Windows
   - Handle multi-display scenarios
   - Return base64-encoded PNG to frontend

3. **Backend - FileService**
   - Implement save dialog
   - Support PNG/JPEG formats
   - Auto-naming with timestamps

4. **Frontend - CaptureWindow**
   - Fullscreen overlay with frozen screenshot
   - Drag-to-select region
   - Emit crop coordinates
   - Client-side image cropping

5. **Frontend - EditorWindow**
   - Konva canvas setup
   - Basic annotation tools:
     - Rectangle
     - Arrow
     - Text
     - Highlight (semi-transparent rect)
   - Undo/Redo functionality
   - Export to PNG

6. **Integration**
   - Wire backend services to frontend
   - Test capture → edit → save flow
   - Handle errors gracefully

---

### Phase 2: Advanced Features (Week 3-4)
**Goal**: Upload, hotkeys, system tray

#### Tasks:
1. **Backend - UploadService**
   - Define provider interface
   - Implement local provider (copy to clipboard)
   - Implement HTTP upload provider (generic POST)
   - Add provider configuration

2. **Backend - HotkeyService**
   - Global hotkey registration (macOS)
   - Global hotkey registration (Windows)
   - Trigger capture flows from hotkeys
   - Handle permission requests (macOS Screen Recording)

3. **Backend - SettingsService**
   - JSON-based config file
   - Load/save settings
   - Default hotkey bindings
   - Upload provider config

4. **Frontend - Settings UI**
   - Hotkey customization
   - Upload provider selection
   - Default save path
   - Format preferences

5. **Frontend - System Tray**
   - Tray icon integration
   - Context menu (Capture, Settings, Quit)
   - Show/hide main window

6. **Testing**
   - Unit tests for all services (≥50% coverage)
   - Integration tests for capture flow
   - Manual testing on macOS + Windows

---

### Phase 3: Polish & Distribution (Week 5-6)
**Goal**: Production-ready app with installers

#### Tasks:
1. **Performance Optimization**
   - Optimize image encoding/decoding
   - Reduce memory footprint
   - Fast startup time (<2s)

2. **Error Handling & UX**
   - Permission error dialogs (macOS)
   - Network error handling (upload)
   - Loading states and progress indicators
   - Toast notifications

3. **Documentation**
   - User guide (README.md)
   - API documentation (godoc)
   - Build instructions
   - Troubleshooting guide

4. **Build & Distribution**
   - macOS: Code signing + notarization
   - Windows: Code signing (optional)
   - Create installers (.dmg, .exe)
   - Setup auto-update mechanism (optional)

5. **CI/CD**
   - GitHub Actions for builds
   - Automated testing
   - Release automation

---

## 5. Technical Decisions & Rationale

### 5.1 Why Wails?
- **Native performance**: Go backend for OS integration
- **Modern UI**: Web stack for rapid iteration
- **Small binary**: ~10-20MB vs Electron's 100MB+
- **Cross-platform**: Single codebase for macOS + Windows

### 5.2 Why Konva.js?
- **Scene graph**: Easy object manipulation
- **Built-in transforms**: Resize, rotate, drag
- **Undo/Redo**: JSON state snapshots
- **Performance**: Hardware-accelerated canvas

### 5.3 Why Zustand?
- **Lightweight**: <1KB, no boilerplate
- **Simple API**: Easy to learn and use
- **TypeScript**: First-class TS support
- **No Context hell**: Direct store access

### 5.4 Client-side vs Server-side Cropping
- **Client-side chosen**: Faster UX, no round-trip
- Canvas API can crop efficiently
- Reduces backend complexity

---

## 6. Platform-Specific Considerations

### 6.1 macOS
**Permissions**:
- Screen Recording permission required (TCC)
- Detect permission status via CGPreflightScreenCaptureAccess
- Show user guidance if denied

**Distribution**:
- Code signing with Apple Developer ID
- Notarization for Gatekeeper
- DMG installer with drag-to-Applications

**Hotkeys**:
- Use Carbon API or CGEvent tap
- Requires Accessibility permission for global hotkeys

### 6.2 Windows
**DPI Scaling**:
- Handle high-DPI displays correctly
- Normalize coordinates across monitors
- Use SetProcessDPIAware

**Hotkeys**:
- RegisterHotKey Win32 API
- Handle WM_HOTKEY messages

**Distribution**:
- NSIS or WiX installer
- Optional code signing with Authenticode
- Add to Windows Defender exclusions if needed

---

## 7. Testing Strategy

### 7.1 Unit Tests (Go)
- All service interfaces mocked with mockery
- Test coverage ≥50%
- Focus on business logic and error handling

### 7.2 Integration Tests
- Test full capture → edit → save flow
- Mock OS-specific calls where needed
- Use testify/assert for assertions

### 7.3 Manual Testing Checklist
- [ ] Screenshot capture on single display
- [ ] Screenshot capture on multi-display
- [ ] Region selection accuracy
- [ ] All annotation tools work
- [ ] Undo/Redo functionality
- [ ] Save to disk (PNG, JPEG)
- [ ] Upload to provider
- [ ] Global hotkeys trigger capture
- [ ] System tray menu works
- [ ] Settings persistence
- [ ] Permission handling (macOS)
- [ ] DPI scaling (Windows)

---

## 8. Development Workflow

### 8.1 Setup
```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install dependencies
go mod download
cd frontend && npm install

# Generate mocks
make mocks
```

### 8.2 Development
```bash
# Run in dev mode (hot reload)
wails dev

# Run tests
make test

# Lint
make lint
```

### 8.3 Build
```bash
# Build for current platform
wails build

# Build for specific platform
wails build -platform darwin/amd64
wails build -platform windows/amd64
```

---

## 9. Risk Mitigation

### 9.1 Technical Risks
| Risk | Impact | Mitigation |
|------|--------|------------|
| Screenshot library doesn't support multi-display | High | Test early, fallback to platform-specific code |
| Global hotkeys conflict with system | Medium | Allow customization, detect conflicts |
| macOS permission denial | High | Clear user guidance, detect and prompt |
| Canvas performance on large images | Medium | Optimize rendering, limit max resolution |

### 9.2 Timeline Risks
| Risk | Impact | Mitigation |
|------|--------|------------|
| Platform-specific bugs take longer | Medium | Allocate buffer time in Phase 3 |
| Code signing delays | Low | Start early, use test certificates |
| Konva learning curve | Low | Prototype early, use examples |

---

## 10. Success Metrics

### MVP Success Criteria
- [ ] Capture screenshot in <500ms
- [ ] Smooth annotation (60fps)
- [ ] Save image in <1s
- [ ] Binary size <25MB
- [ ] Startup time <2s
- [ ] Works on macOS 11+ and Windows 10+

### Phase 2 Success Criteria
- [ ] Global hotkeys work reliably
- [ ] Upload completes in <3s
- [ ] Settings persist correctly
- [ ] System tray integrates smoothly

### Phase 3 Success Criteria
- [ ] Installers work on fresh systems
- [ ] No crashes in 1-hour stress test
- [ ] All manual tests pass
- [ ] Documentation complete

---

## 11. Next Steps

1. **Initialize Wails project**: `wails init -n grabix -t react-ts`
2. **Setup project structure**: Create internal/ and service/ directories
3. **Configure Makefile**: Add common commands (test, lint, mocks, build)
4. **Implement CaptureService**: Start with macOS, then Windows
5. **Build CaptureWindow**: Fullscreen overlay with region selection
6. **Iterate**: Test early, test often

---

## 12. Known Issues & Solutions

### 12.1 macOS Tray Icon Not Showing (RESOLVED)

**Issue**: NSStatusItem được tạo nhưng không hiển thị trên menu bar.

**Root Cause**:
- NSStatusItem được tạo quá sớm trong NSApplication lifecycle
- Khi tạo status item trước khi `NSApplicationDidFinishLaunching`, icon không được attach vào menu bar
- Không crash, không có error log, nhưng icon không hiển thị

**Symptoms**:
- ✅ NSStatusItem created successfully
- ✅ NSStatusBarWindow exists và visible
- ✅ Button frame có size hợp lệ
- ❌ Icon không xuất hiện trên menu bar

**Solution**:
1. **Defer tray creation** với `dispatch_after` 300ms delay trên main queue
2. **Retain objects** với `CFRetain` để tránh ARC deallocation
3. **Không đụng tới `statusItem.button.window`** - để macOS tự quản lý

**Implementation** (`internal/tray/tray_darwin.m`):
```objc
void tray_init(void) {
    @autoreleasepool {
        NSLog(@"[Grabix] Scheduling tray creation after app launch...");

        // Defer tray creation until after NSApplication finishes launching
        dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(0.3 * NSEC_PER_SEC)),
                      dispatch_get_main_queue(), ^{
            create_status_item();
        });
    }
}

static void create_status_item(void) {
    @autoreleasepool {
        // Create delegate and retain
        if (delegate == nil) {
            delegate = [[TrayDelegate alloc] init];
            CFRetain((__bridge CFTypeRef)delegate);
        }

        // Create status item and retain
        statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
        CFRetain((__bridge CFTypeRef)statusItem);

        // Set title and menu
        statusItem.button.title = @"📸 Grabix";
        statusItem.menu = menu;

        // DON'T touch statusItem.button.window - let macOS manage it
    }
}
```

**Key Learnings**:
- Wails v2 không có built-in tray API
- `systray` library conflict với Wails (duplicate AppDelegate symbols)
- NSStatusItem lifecycle phụ thuộc vào NSApplication launch state
- Timing issue không gây crash nhưng gây silent failure

**References**:
- Wails GitHub Issue #1010: "Running systray and Wails together is basically impossible"
- Wails v3 changelog: "Fix MacOS systray click handling when no attached window"

**Date Resolved**: 2025-12-20

---

**Document Version**: 1.1
**Last Updated**: 2025-12-20
**Owner**: Xếp


