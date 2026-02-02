# System Architecture - Grabix

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    User (macOS/Windows)                  │
├─────────────────────────────────────────────────────────┤
│                  Grabix Desktop App                       │
│  ┌──────────────────────────────────────────────────┐   │
│  │         Frontend (React + TypeScript)             │   │
│  │  - UI Components (Editor, Settings, Toast)       │   │
│  │  - Konva Canvas (annotation rendering)           │   │
│  │  - Zustand Stores (state management)             │   │
│  │  - Wails JS bindings                             │   │
│  └────────────────▲─────────────────────────────────┘   │
│                   │ IPC (JSON/Binary)                    │
│  ┌────────────────▼─────────────────────────────────┐   │
│  │         Backend (Go + Wails v2.11)               │   │
│  │  - Service Layer (Capture, File, Hotkey, etc)   │   │
│  │  - OS Integration (permissions, clipboard)       │   │
│  │  - Settings Management                           │   │
│  └────────────────▲─────────────────────────────────┘   │
│                   │                                      │
├───────────────────┼──────────────────────────────────────┤
│  OS APIs          │                                      │
│  - CGImage        │ System Tray, Hotkeys, Screenshot    │
│  - Clipboard      │ Permissions, File dialogs           │
│  - Permissions    │                                      │
└───────────────────┼──────────────────────────────────────┘
                    │
            ┌───────┴────────┐
            │                │
        ┌─────────┐      ┌─────────┐
        │  macOS  │      │ Windows │
        │  APIs   │      │  APIs   │
        └─────────┘      └─────────┘
```

---

## 2. Backend Architecture

### 2.1 Service-Oriented Design

Backend organized as independent services, each with a clean interface:

```
internal/service/
├── capture/          # Screenshot capture (darwin/windows)
│   ├── service.go    # Interface definition
│   ├── capture_darwin.go
│   └── capture_windows.go
├── clipboard/        # Copy to clipboard
├── file/             # Save dialog, file I/O
├── hotkey/           # Global hotkey registration
├── permission/       # OS permission checks
├── settings/         # Configuration persistence
├── startup/          # Run at startup (darwin)
└── upload/           # Future: cloud uploads
```

Each service exports a **single interface** with methods, not a dependency-heavy constructor:

```go
// Example: CaptureService interface
type Service interface {
    CaptureFullscreen(ctx context.Context) ([]byte, error)
    CaptureActiveDisplay(ctx context.Context) ([]byte, error)
    GetDisplayInfo(ctx context.Context) ([]DisplayInfo, error)
}

// Used as
captureService := capture.New()
imageBytes, err := captureService.CaptureFullscreen(ctx)
```

### 2.2 Core Services

#### CaptureService
**Purpose**: Screenshot operations

**Methods**:
- `CaptureFullscreen()` → PNG bytes
- `CaptureActiveDisplay()` → PNG bytes of current monitor
- `GetDisplayInfo()` → Display list with coordinates

**Platform-Specific**:
- **macOS**: Uses CGDisplayCreateImage (from CoreGraphics)
- **Windows**: Uses Win32 GDI (GetDC, BitBlt)

**Returns**: Base64-encoded PNG for frontend rendering

#### ClipboardService
**Purpose**: Copy data to system clipboard

**Methods**:
- `Copy(data []byte, format string)` → error

**Platform-Specific**:
- **macOS**: `pbcopy` command via exec
- **Windows**: Win32 SetClipboardData

**Formats**: PNG binary, text, HTML (for future)

#### FileService
**Purpose**: Save dialog and file I/O

**Methods**:
- `OpenSaveDialog(defaultName string)` → string (path), error
- `SaveImage(path, format, quality int, data []byte)` → error
- `GetDefaultSavePath()` → string, error

**Formats**: PNG (lossless), JPEG (with quality param), WebP

**Auto-naming**: Timestamps + display index
```
Screenshot_20250201_143022_Display1.png
```

#### HotkeyService
**Purpose**: Global hotkey registration (works from any app)

**Methods**:
- `Register(key string, callback func())` → error
- `Unregister(key string)` → error
- `IsRegistered(key string)` → bool

**Key Format**: Platform-specific
- macOS: `"cmd+shift+s"` or `"ctrl+shift+s"`
- Windows: `"ctrl+shift+s"` or `"alt+shift+s"`

**Platform-Specific**:
- **macOS**: uses `golang.design/x/hotkey` (CGEventTap)
- **Windows**: uses `golang.design/x/hotkey` (RegisterHotKey Win32 API)

#### PermissionService
**Purpose**: Check and request OS permissions

**Methods**:
- `CheckScreenRecordingPermission()` → bool
- `RequestScreenRecordingPermission()` → error
- `IsAccessibilityEnabled()` → bool

**macOS-Specific**: Screen Recording permission (TCC database)
- Checks `/Library/Application Support/com.apple.sharedfilelist/`
- Prompts user if missing

**Windows**: Accessibility permissions (similar to accessibility consent)

#### SettingsService
**Purpose**: Persistent configuration (JSON file)

**Methods**:
- `Load()` → error
- `Save()` → error
- `Get(key string)` → interface{}, error
- `Set(key string, value interface{})` → error

**Location**: `~/.grabix/settings.json`

**Structure**:
```json
{
  "hotkey": "cmd+shift+s",
  "format": "png",
  "quality": 95,
  "savePath": "~/Pictures",
  "fontSize": 16,
  "colors": { "rect": "#FF0000" }
}
```

#### StartupService (macOS only)
**Purpose**: Register app to run at login

**Methods**:
- `Enable()` → error
- `Disable()` → error
- `IsEnabled()` → bool, error

**Implementation**: Uses LaunchAgent plist file in `~/Library/LaunchAgents/`

#### UploadService (Future)
**Purpose**: Cloud uploads (extensible)

**Methods**:
- `Upload(data []byte, filename string)` → *UploadResult, error
- `GetProviders()` → []string
- `SetProvider(name string)` → error

**Provider Interface**:
```go
type Provider interface {
    Upload(ctx context.Context, data []byte) (*Result, error)
}
```

Planned providers: Imgur, S3, Dropbox, HTTP custom

### 2.3 Data Models

Core types in `internal/model/types.go`:

```go
// CaptureResult holds screenshot data
type CaptureResult struct {
    Data    string // base64-encoded PNG
    Width   int
    Height  int
    DPI     float64
}

// DisplayInfo describes a monitor
type DisplayInfo struct {
    ID     string
    Name   string
    Bounds Rect   // X, Y, Width, Height
    Scale  float64 // DPI scale factor
}

// Annotation represents a drawn shape
type Annotation struct {
    ID       string // UUID
    Type     string // "rectangle", "arrow", "text", "highlight"
    Color    string // Hex: #RRGGBB
    Alpha    float64 // 0-1
    Bounds   Rect
    Text     string // For text annotations
    StrokeWidth int
}

// Settings holds user configuration
type Settings struct {
    Hotkey        string
    Format        string // "png", "jpeg", "webp"
    Quality       int    // 0-100, for JPEG/WebP
    SavePath      string
    StartupEnabled bool
}
```

---

## 3. Frontend Architecture

### 3.1 Component Hierarchy

```
App.tsx
├── CaptureWindow/
│   ├── RegionSelector (drag to select region)
│   └── CrosshairCursor
├── EditorWindow/
│   ├── Toolbar (tool selection)
│   ├── Canvas (Konva stage)
│   ├── ActionBar (Save, Copy, Upload buttons)
│   └── ZoomBar (zoom controls)
├── SettingsWindow/
│   ├── HotkeyInput
│   ├── FormatSelector
│   └── PathSelector
├── PermissionWarning/ (if permission denied)
└── Toast/ (notifications)
```

### 3.2 State Management (Zustand)

Two main stores:

#### editorStore.ts (225 LOC)
Manages canvas state and annotations:

```typescript
interface EditorState {
    // Data
    imageData: Blob | null
    imageWidth: number
    imageHeight: number
    annotations: Annotation[]
    selectedTool: AnnotationTool
    selectedColor: string
    strokeWidth: number

    // History
    undoStack: Annotation[][]
    redoStack: Annotation[][]

    // UI State
    zoomLevel: number
    panX: number
    panY: number
    isLoading: boolean

    // Actions
    setImage: (blob: Blob, w: number, h: number) => void
    addAnnotation: (anno: Annotation) => void
    deleteAnnotation: (id: string) => void
    undo: () => void
    redo: () => void
    setTool: (tool: AnnotationTool) => void
    setColor: (color: string) => void
    reset: () => void
}

export const useEditorStore = create<EditorState>((set) => ({...}))
```

#### captureStore.ts
Manages capture state and settings:

```typescript
interface CaptureState {
    // Last capture
    lastImageBase64: string | null
    lastImageSize: { width: number; height: number }

    // Settings
    hotkey: string
    format: 'png' | 'jpeg' | 'webp'
    savePath: string
    fontSize: number

    // Actions
    setCapture: (imageData: string, size) => void
    loadSettings: () => Promise<void>
    saveSettings: () => Promise<void>
}

export const useCaptureStore = create<CaptureState>((set) => ({...}))
```

**Pattern**: Actions are methods on store, not separate dispatchers
```typescript
const store = useCaptureStore()
store.setCapture(imageBase64, size) // Direct call, no dispatch
```

### 3.3 Key Components

#### EditorWindow.tsx (1257 LOC - needs splitting)

Main canvas editor component:
- **Canvas**: Konva stage with image + annotation layers
- **Toolbar**: 4 tools (rectangle, arrow, text, highlight)
- **ActionBar**: Save, Copy, Upload buttons
- **ZoomBar**: 25%, 50%, 75%, 100%, 150%, 200% zoom levels

**Konva Structure**:
```typescript
<Stage width={width} height={height}>
    <Layer name="image">
        <Image image={canvasImage} />
    </Layer>
    <Layer name="annotations">
        {annotations.map(anno => (
            anno.type === 'rectangle' ? <Rect {...} /> :
            anno.type === 'text' ? <Text {...} /> :
            // ... etc
        ))}
    </Layer>
    <Layer name="selection">
        {selectedAnnotationId && <Rect {...} />}
    </Layer>
</Stage>
```

**Mouse Events**:
- `onMouseDown`: Start drawing new annotation or select existing
- `onMouseMove`: Update annotation bounds in real-time
- `onMouseUp`: Finalize annotation, push to undo stack

#### SettingsWindow.tsx
Configuration dialog:
- Hotkey input with conflict detection
- Format selection (PNG/JPEG/WebP)
- Save path picker
- Startup toggle

#### Toast.tsx
Notification system:
```typescript
useToast('Saved to ~/Pictures/screenshot.png', 'success')
useToast('Failed to upload', 'error')
```

### 3.4 Custom Hooks

Reusable logic extracted to hooks:

- `useKeyDown(key)` - Listen for keyboard shortcuts
- `useHotkey()` - Trigger captures from hotkey
- `useImage()` - Load/process image data
- `useEditorUndo()` - Undo/redo keyboard shortcuts
- `useClipboard()` - Copy to clipboard integration

### 3.5 Wails Integration

Services wrapper around Wails invocations:

```typescript
// services/capture-service.ts
export const captureService = {
    fullscreen: async (): Promise<CaptureResult> => {
        return window.wails.Invoke('CaptureService.FullScreen')
    },

    activeDisplay: async (): Promise<CaptureResult> => {
        return window.wails.Invoke('CaptureService.ActiveDisplay')
    }
}

// Usage in component
const image = await captureService.fullscreen()
useEditorStore.setState({ imageData: image })
```

---

## 4. Data Flow

### 4.1 Capture → Edit → Save Flow

```
User presses hotkey (Cmd+Shift+S)
    ↓
HotkeyService.Register() triggered callback
    ↓
Frontend receives hotkey event via Wails
    ↓
CaptureService.CaptureActiveDisplay() called
    ↓
Returns base64-encoded PNG to frontend
    ↓
EditorWindow opens with image rendered on Konva canvas
    ↓
User draws annotations (rectangles, arrows, text)
    ↓
Annotations stored in editorStore
    ↓
User clicks "Save"
    ↓
Canvas.toDataURL() exports annotated image as PNG blob
    ↓
Frontend calls FileService.SaveImage(path, format, quality, blob)
    ↓
Backend writes file to disk
    ↓
Toast notification: "Saved to ~/Pictures/..."
```

### 4.2 Settings Persistence Flow

```
User opens SettingsWindow
    ↓
Load current settings from SettingsService.Load()
    ↓
Display in UI (hotkey input, format selector, etc)
    ↓
User modifies settings
    ↓
Click "Save"
    ↓
SettingsService.Set(key, value) updates in-memory
    ↓
SettingsService.Save() writes to ~/.grabix/settings.json
    ↓
HotkeyService.Unregister() old hotkey
    ↓
HotkeyService.Register() new hotkey
    ↓
Toast: "Settings saved"
```

### 4.3 Permission Check Flow (macOS)

```
App startup
    ↓
PermissionService.CheckScreenRecordingPermission()
    ↓
If denied:
    ↓
Show PermissionWarning component
    ↓
User clicks "Open System Preferences"
    ↓
Launch System Preferences → Privacy & Security → Screen Recording
    ↓
User adds Grabix to allowed apps
    ↓
App restarts (or user click "Continue")
    ↓
Re-check permission, proceed if granted
```

---

## 5. Cross-Platform Implementation

### 5.1 Build Tags Strategy

Files organized with platform-specific variants:

```
capture/
├── service.go              # Interface (all platforms)
├── capture_darwin.go       // +build darwin
├── capture_windows.go      // +build windows
├── common.go               # Shared utilities
```

Go build system automatically selects based on target:
```bash
GOOS=darwin go build      # Uses capture_darwin.go
GOOS=windows go build     # Uses capture_windows.go
```

### 5.2 Platform-Specific Quirks

#### macOS
- **Screenshot**: Uses CGDisplayCreateImage (CoreGraphics)
- **Permissions**: TCC database check for Screen Recording
- **Hotkeys**: CGEventTap (requires Accessibility permission)
- **Tray**: NSStatusItem (macOS menu bar)
- **Clipboard**: pbcopy/pbpaste commands
- **Startup**: LaunchAgent plist in ~/Library/LaunchAgents/

#### Windows
- **Screenshot**: Win32 GDI (GetDC, BitBlt, CreateCompatibleDC)
- **DPI**: Multiple monitor DPI scaling normalization
- **Hotkeys**: RegisterHotKey Win32 API
- **Tray**: System tray icon (taskbar)
- **Clipboard**: Win32 SetClipboardData/GetClipboardData
- **Startup**: Registry HKLM Run key or Task Scheduler

### 5.3 Conditional Compilation (Frontend)

No OS detection needed in React - backend handles it.

For UI differences:
```typescript
const isMac = navigator.platform.startsWith('Mac')

return isMac ? (
    <Tooltip label="Cmd+Shift+S">...</Tooltip>
) : (
    <Tooltip label="Ctrl+Shift+S">...</Tooltip>
)
```

---

## 6. Communication Protocol

### IPC (Inter-Process Communication) via Wails

**Frontend → Backend** (calling Go functions):
```typescript
// Invoke Go method
const result = await window.wails.Invoke('ServiceName.MethodName', arg1, arg2)
```

**Backend → Frontend** (sending events):
```go
// Emit event from Go
wails.EventsEmit("eventName", data)

// Listen in React
useEffect(() => {
    const unlisten = window.wails.EventsOn('eventName', (data) => {
        // Handle event
    })
    return unlisten
}, [])
```

**Message Format**: JSON-serialized
- Arguments encoded as JSON
- Return values decoded from JSON
- Base64 for binary data (images)

**Error Handling**:
```typescript
try {
    const result = await window.wails.Invoke('Service.Method', arg)
} catch (error) {
    // error is a string from Go
    console.error('Go error:', error)
}
```

---

## 7. Deployment Architecture

### Build Output
- **macOS**: `grabix.app/` (self-contained bundle)
- **Windows**: `grabix.exe` (single executable)

### Distribution
- **macOS**: `.dmg` (disk image with drag-to-Applications)
- **Windows**: `.exe` installer (NSIS or MSI)

### Code Signing
- **macOS**: Developer ID certificate + notarization (Apple)
- **Windows**: Optional Authenticode certificate

### Auto-Update (Future)
- Wails has built-in auto-update support
- Can use GitHub releases or custom server

---

## 8. Performance Characteristics

### Capture Performance
| Operation | Target | Actual |
|-----------|--------|--------|
| Screenshot capture | <500ms | ~200-400ms |
| Annotation render | 60 FPS | ✓ Smooth |
| Save to disk | <1s | ~200-500ms |
| App startup | <2s | ~1.2s |

### Memory Usage
- **Idle**: ~30MB
- **With 2000x1500 image**: ~50MB
- **With 10 annotations**: ~55MB

### Binary Size
- **macOS universal**: ~15MB
- **Windows**: ~12MB
- Small for a cross-platform desktop app

---

## 9. Security Considerations

### Data Handling
- Images stored temporarily in memory, not on disk (except user saves)
- Clipboard operations are ephemeral
- No telemetry or phone-home

### Permissions
- Only requests permissions when needed (screenshot, hotkeys, accessibility)
- Clear user dialogs explain why permission needed
- Can be revoked anytime (no persistent elevation)

### Code Signing
- macOS: Signed with Developer ID
- Windows: Can be signed with Authenticode (optional)
- Prevents tampering and gatekeeper warnings

---

**Version**: 1.0
**Last Updated**: 2025-02-01
