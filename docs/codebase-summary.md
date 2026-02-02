# Grabix - Codebase Summary

**Generated from**: repomix analysis
**Total Files**: 156 (excluding node_modules)
**Total Lines**: ~4,270 (frontend), ~1,000+ (backend)
**Languages**: Go, TypeScript, React, CSS

---

## 1. Project Metrics

### Size Breakdown
- **Go Backend**: ~1,000 LOC across services
- **React Frontend**: ~4,270 LOC total
- **Styles**: ~500 LOC CSS
- **Config**: JSON, Makefile, shell scripts
- **Docs**: ~500 LOC

### Largest Files
1. **EditorWindow.tsx** - 1,257 LOC (canvas editor component)
2. **editorStore.ts** - 225 LOC (Zustand store)
3. **technical-plan.md** - 540 LOC (design document)
4. **design.md** - 210 LOC (architecture document)

### Component Count
- **React Components**: 8+ main components
- **Custom Hooks**: 5+ hooks
- **Zustand Stores**: 2 stores (editor, capture)
- **Go Services**: 8 services (capture, file, hotkey, etc.)

---

## 2. Backend Structure (Go)

### Main Entry Point
**`main.go` (387 LOC)**
- Wails app initialization
- Service instantiation
- Exposed API bindings to frontend
- Event emission setup

Key exports to frontend:
```go
// CaptureService
wails.Invoke("CaptureService.FullScreen")
wails.Invoke("CaptureService.ActiveDisplay")

// FileService
wails.Invoke("FileService.SaveImage")
wails.Invoke("FileService.OpenSaveDialog")

// SettingsService
wails.Invoke("SettingsService.Load")
wails.Invoke("SettingsService.Save")

// HotkeyService
wails.Invoke("HotkeyService.Register")
wails.Invoke("HotkeyService.Unregister")

// ClipboardService
wails.Invoke("ClipboardService.Copy")

// PermissionService
wails.Invoke("PermissionService.CheckScreenRecording")

// StartupService (macOS)
wails.Invoke("StartupService.Enable")
wails.Invoke("StartupService.Disable")
```

### Data Models
**`internal/model/types.go` (91 LOC)**

Core types:
- `CaptureResult` - Screenshot output (base64 data, dimensions)
- `DisplayInfo` - Monitor information (ID, bounds, DPI)
- `Annotation` - Shape/text on canvas
- `Settings` - User configuration
- `UploadResult` - Upload response

### Service Modules

#### 1. Capture Service
**Path**: `internal/service/capture/`
- **service.go** - Interface definition
- **capture_darwin.go** - macOS CGImage implementation
- **capture_windows.go** - Windows GDI implementation

Functions:
- `CaptureFullscreen()` - All displays
- `CaptureActiveDisplay()` - Current monitor only
- `GetDisplayInfo()` - List monitors with bounds

#### 2. Clipboard Service
**Path**: `internal/service/clipboard/`
- **clipboard_darwin.go** - `pbcopy` command
- **clipboard_windows.go** - Win32 SetClipboardData

Functions:
- `Copy(data []byte, format string)` - Write to clipboard

#### 3. File Service
**Path**: `internal/service/file/`
- **service.go** - Interface + platform-specific dialogs
- **file_darwin.go** - macOS NSOpenPanel
- **file_windows.go** - Windows Shell API

Functions:
- `OpenSaveDialog()` - Native file picker
- `SaveImage()` - Write PNG/JPEG/WebP to disk
- `GetDefaultSavePath()` - Platform defaults

#### 4. Hotkey Service
**Path**: `internal/service/hotkey/`
- **service.go** - Interface
- **hotkey_darwin.go** - golang.design/x/hotkey (CGEventTap)
- **hotkey_windows.go** - golang.design/x/hotkey (RegisterHotKey)

Functions:
- `Register(key, callback)` - Listen for global hotkey
- `Unregister(key)` - Stop listening
- `IsRegistered(key)` - Check registration status

#### 5. Permission Service
**Path**: `internal/service/permission/`
- **permission_darwin.go** - Screen Recording TCC check
- **permission_windows.go** - Accessibility consent

Functions:
- `CheckScreenRecording()` - Verify permission granted
- `RequestScreenRecording()` - Prompt user (macOS)

#### 6. Settings Service
**Path**: `internal/service/settings/`
- **service.go** - JSON config management
- Reads/writes `~/.grabix/settings.json`

Functions:
- `Load()` - Read from disk
- `Save()` - Write to disk
- `Get(key)` - Retrieve setting value
- `Set(key, value)` - Update setting

#### 7. Startup Service (macOS only)
**Path**: `internal/service/startup/`
- **startup_darwin.go** - LaunchAgent plist creation
- **startup_windows.go** - Stub (not implemented)

Functions:
- `Enable()` - Add to ~/Library/LaunchAgents/
- `Disable()` - Remove plist
- `IsEnabled()` - Check registration

#### 8. Upload Service (Future)
**Path**: `internal/service/upload/`
- **service.go** - Interface definition (placeholder)
- Provider pattern for extensibility

### Tray Integration (macOS)
**Path**: `internal/tray/`
- **tray.go** - Interface definition
- **tray_darwin.go** - NSStatusItem implementation (Objective-C wrapper)

Known issue (resolved in v1.1): Tray icon visibility timing on Big Sur
- Solution: Defer NSStatusItem creation 300ms after app launch
- Use CFRetain to prevent ARC deallocation

### Version Management
**Path**: `internal/version/`
- **version.go** - Version constants
- Used for About dialogs, release info

---

## 3. Frontend Structure (React + TypeScript)

### Entry Point
**`frontend/src/main.tsx` (50 LOC)**
- React 18 app initialization
- Mounts App.tsx to #app DOM element
- Vite module setup

### Main App Component
**`frontend/src/App.tsx` (150 LOC)**
- Routes/layout for different windows
- Global state initialization
- Permission warning component

### Components Directory

#### EditorWindow (1257 LOC - LARGEST)
**Path**: `frontend/src/components/EditorWindow/`
- **EditorWindow.tsx** - Main component, 1257 LOC
- **Toolbar.tsx** - Tool selection (rect, arrow, text, highlight)
- **ActionBar.tsx** - Save, Copy, Upload buttons
- **ZoomBar.tsx** - Zoom controls (25%-200%)
- **EditorWindow.module.css** - Component styles

**Key Features**:
- Konva canvas for annotation rendering
- Mouse event handlers for drawing
- Undo/redo with keyboard shortcuts
- Export to PNG/JPEG/WebP
- Real-time zoom and pan

**Dependency**: Heavy - uses Konva.js directly
- Todo: Split into smaller sub-components (<200 LOC each)

#### SettingsWindow
**Path**: `frontend/src/components/SettingsWindow/`
- Settings UI (hotkey input, format selector, path picker)
- Load/save settings via wails.Invoke
- Validation and error handling

#### PermissionWarning
- Conditional display when permission denied (macOS)
- Links to System Preferences
- Retry button to re-check permission

#### Toast
- Notification system for user feedback
- Success/error/info states
- Auto-dismiss after 3s

### State Management (Zustand)

#### editorStore.ts (225 LOC)
```typescript
interface EditorState {
    // Image data
    imageData: Blob | null
    imageWidth: number
    imageHeight: number

    // Annotations
    annotations: Annotation[]
    selectedTool: AnnotationTool
    selectedColor: string
    strokeWidth: number

    // History
    undoStack: Annotation[][]
    redoStack: Annotation[][]

    // UI
    zoomLevel: number
    panX: number
    panY: number
    isLoading: boolean

    // Methods
    setImage(blob, w, h): void
    addAnnotation(anno): void
    deleteAnnotation(id): void
    undo(): void
    redo(): void
    setTool(tool): void
    setColor(color): void
}
```

#### captureStore.ts
- Last captured image (base64)
- Settings (hotkey, format, savePath)
- Load/save settings from backend

### Custom Hooks
**Path**: `frontend/src/hooks/`

- `useKeyDown(key, callback)` - Keyboard event listener
- `useHotkey()` - Global hotkey trigger
- `useImage()` - Image loading/processing
- `useEditorUndo()` - Undo/redo shortcuts (Ctrl+Z, Ctrl+Y)
- `useClipboard()` - Copy to clipboard

### Services Wrapper
**Path**: `frontend/src/services/`

Type-safe wrappers around Wails.Invoke calls:
```typescript
export const captureService = {
    fullscreen: () => window.wails.Invoke('CaptureService.FullScreen'),
    activeDisplay: () => window.wails.Invoke('CaptureService.ActiveDisplay')
}

export const fileService = {
    openSaveDialog: (name) => window.wails.Invoke('FileService.OpenSaveDialog', name),
    saveImage: (path, format, quality, data) => ...
}
```

### Types Definition
**Path**: `frontend/src/types/index.ts` (88 LOC)**

TypeScript interfaces:
- `CaptureResult` - Screenshot metadata
- `Annotation` - Shape/text object
- `DisplayInfo` - Monitor info
- `AnnotationTool` - Union type of tools
- `Settings` - User config

### Styling
- **CSS Modules**: Component-scoped styles
- **No CSS framework** (plain CSS)
- **Responsive design**: Mobile-first approach

Key CSS patterns:
- `.container` - Main layout
- `.toolbar` - Tool selection UI
- `.canvas` - Konva stage container
- `.button` - Consistent button styling

---

## 4. Configuration Files

### wails.json
```json
{
  "name": "grabix",
  "outputfilename": "grabix",
  "wailsjsdir": "./frontend",
  "frontend:dir": "./frontend",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto"
}
```

### go.mod
```
module github.com/heytonyne/grabix
go 1.24

require github.com/wailsapp/wails/v2 v2.11.0
require golang.design/x/hotkey v0.4.1
```

Dependencies: Minimal (2 main libraries)
- Wails v2.11 for framework
- golang.design/x/hotkey for global hotkeys

### package.json
```json
{
  "name": "grabix-frontend",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  }
}
```

**Dependencies**:
- react 18.x
- react-dom 18.x
- zustand (state management)
- konva + react-konva (canvas)
- typescript (type checking)

**DevDependencies**:
- vite (bundler)
- @vitejs/plugin-react
- @types/react, @types/react-dom
- tailwindcss (optional)

### Makefile
Commands for development:
```bash
make dev           # Run in development mode
make build         # Build production binary
make test          # Run Go tests
make fmt           # Format code
make lint          # Lint code
make mocks         # Generate mocks for testing
```

### .env.example
Configuration template (if needed):
```
WAILS_LOG_LEVEL=debug
WAILS_DEV_SERVER_URL=auto
```

---

## 5. Build System

### Development (wails dev)
- Hot reload for React changes (Vite dev server)
- Go backend auto-restart on file changes
- Browser devtools available at http://localhost:34115

### Production Build (wails build)
**Output**:
- **macOS**: `grabix.app/` self-contained bundle
- **Windows**: `grabix.exe` executable

**Build Targets**:
```bash
wails build                          # Current OS
wails build -platform darwin/amd64   # macOS Intel
wails build -platform darwin/arm64   # macOS Apple Silicon
wails build -platform darwin/universal # macOS Universal
wails build -platform windows/amd64  # Windows 64-bit
```

---

## 6. Key Dependencies & Libraries

### Backend (Go)
| Package | Version | Purpose |
|---------|---------|---------|
| wailsapp/wails | v2.11 | Desktop framework |
| golang.design/x/hotkey | v0.4.1 | Global hotkeys |
| net/http | stdlib | HTTP client (future uploads) |
| image | stdlib | Image processing |
| encoding/base64 | stdlib | Image encoding |

### Frontend (npm)
| Package | Version | Purpose |
|---------|---------|---------|
| react | 18.x | UI framework |
| zustand | latest | State management |
| konva | 9.x | Canvas library |
| react-konva | 18.x | React bindings for Konva |
| typescript | 5.x | Type checking |
| vite | 5.x | Build tool |

---

## 7. Document Structure

### Existing Docs
- **design.md** (210 LOC) - Original product/architecture design
- **technical-plan.md** (540 LOC) - Implementation roadmap and known issues
- **session.md** - Development session notes

### New Docs (This Release)
- **README.md** (212 LOC) - Project overview and quick start
- **project-overview-pdr.md** (400+ LOC) - PDR and requirements
- **code-standards.md** (500+ LOC) - Go/TypeScript conventions
- **system-architecture.md** (600+ LOC) - Detailed architecture
- **codebase-summary.md** (this file) - Component overview
- **project-roadmap.md** (future) - Development phases

---

## 8. Code Statistics

### Go Backend
- **Total Lines**: ~1,000+
- **Largest File**: main.go (387 LOC)
- **Average Service Size**: 100-150 LOC
- **Test Coverage**: ~50% (unit tests for services)
- **Dependencies**: 2 major external packages

### React Frontend
- **Total Lines**: ~4,270
- **Largest Component**: EditorWindow.tsx (1,257 LOC) **[TOO LARGE]**
- **Average Component**: 150-200 LOC
- **State Management**: 225 LOC (editorStore)
- **Custom Hooks**: 100+ LOC total
- **Dependencies**: 20+ npm packages

### Overall
- **TypeScript Coverage**: 100% (strict mode enabled)
- **CSS Coverage**: ~500 LOC across components
- **Documentation**: ~1,500 LOC

---

## 9. Technology Stack Summary

| Layer | Tech | Version | Notes |
|-------|------|---------|-------|
| **Runtime** | Go | 1.24 | macOS + Windows |
| **Framework** | Wails | v2.11 | Desktop bindings |
| **UI Framework** | React | 18 | Component-based |
| **Type System** | TypeScript | 5 | Strict mode |
| **Canvas** | Konva | 9 | Scene graph |
| **State** | Zustand | Latest | Lightweight store |
| **Build Tool** | Vite | 5 | Fast bundling |
| **Package Manager** | npm | Latest | Node dependencies |

---

## 10. Development Workflow

### Setup
```bash
git clone https://github.com/heytonyne/grabix.git
cd grabix
go mod download
cd frontend && npm install && cd ..
```

### Development
```bash
wails dev              # Hot reload both Go and React
go test ./...          # Run backend tests
npm run lint           # Lint frontend
```

### Commit
```bash
git add .
git commit -m "feat: add rectangle annotation tool"
git push origin feature/name
```

### Build Release
```bash
wails build            # Build for current OS
# Creates grabix.app (macOS) or grabix.exe (Windows)
```

---

## 11. Known Issues & TODOs

### Identified in Code
1. **EditorWindow.tsx too large (1257 LOC)**
   - Should split into Toolbar, Canvas, ActionBar, ZoomBar
   - Impact: Harder to maintain, worse hot reload
   - Priority: Medium (Phase 2)

2. **No tests for React components**
   - Missing: Unit tests, integration tests
   - Priority: Medium (Phase 2)

3. **Limited error messages to users**
   - Some backend errors not surfaced well
   - Priority: Low (Phase 3)

4. **No upload service implementation**
   - Interface exists, providers not implemented
   - Priority: Low (Phase 2+)

5. **Windows DPI scaling**
   - Needs multi-monitor coordinate normalization
   - Priority: Medium (Phase 2)

### Platform-Specific
- **macOS**: NSStatusItem tray icon timing (RESOLVED in v1.1)
- **Windows**: High DPI support needs testing
- **Both**: Hotkey conflicts need better detection

---

## 12. Recommended Next Steps

### Phase 2 (Priority Order)
1. Split EditorWindow.tsx into 4 sub-components
2. Add React component unit tests (Jest + RTL)
3. Implement hotkey customization UI
4. Implement upload service with HTTP provider
5. Add keyboard shortcuts in editor (Ctrl+S, Ctrl+A)

### Phase 3
1. Code signing + notarization (macOS)
2. Installer creation (.dmg, .exe)
3. Auto-update system
4. CI/CD pipeline (GitHub Actions)
5. Performance optimization

---

## 13. File Inventory

### Core Files
```
grabix/
├── main.go                    # Entry point (387 LOC)
├── go.mod / go.sum            # Dependencies
├── wails.json                 # Wails config
├── Makefile                   # Commands
├── internal/
│   ├── model/types.go         # Data models (91 LOC)
│   ├── service/               # 8 services, 800+ LOC total
│   ├── tray/                  # macOS tray
│   └── version/               # Version info
├── frontend/
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/        # 5+ components, 4000+ LOC
│   │   ├── store/             # 2 stores
│   │   ├── hooks/             # 5+ custom hooks
│   │   ├── services/          # Wails wrappers
│   │   └── types/             # TypeScript types
│   ├── package.json
│   └── tsconfig.json
├── build/                     # Icons, platform config
├── scripts/                   # Build helpers
├── docs/                      # Documentation (~1500 LOC)
└── README.md                  # Project overview
```

---

**Version**: 1.0
**Generated**: 2025-02-01
**Analysis Tool**: repomix v0.3.7
**Status**: MVP Complete, Ready for Phase 2
