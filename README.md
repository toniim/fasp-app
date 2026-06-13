# Fasp - Cross-Platform Screenshot & Annotation Tool

Fasp is a lightweight, fast desktop screenshot and annotation tool for macOS and Windows. Capture, annotate with shapes/arrows/text, and save or share in seconds.

## Features

- **Screenshot Capture**: Fullscreen or active display
- **Region Selection**: Freeze screen and select area with crosshair
- **Annotation Tools**:
  - Rectangle, arrow, highlight, text
  - Undo/redo support
  - Adjustable colors and transparency
- **Save & Share**: PNG/JPEG/WebP formats or copy to clipboard
- **Global Hotkeys**: Customizable keyboard shortcuts
- **System Tray**: Quick access from menu bar/system tray
- **Cross-Platform**: Native support for macOS and Windows
- **Permissions**: Smart handling of OS-specific permissions

## Requirements

- **macOS**: 11.0+ (Intel or Apple Silicon)
- **Windows**: 10 (build 1909)+
- **Go**: 1.24+ (for development)
- **Node.js**: 18+ (for frontend development)

## Quick Start

### Installation

**macOS** (via release):
```bash
curl -L https://github.com/heytonyne/fasp/releases/latest/download/fasp.dmg -o fasp.dmg
# Mount and drag to Applications
```

**Windows** (via release):
```bash
# Download fasp-setup.exe from releases
# Run installer
```

### Development Setup

```bash
# Clone repository
git clone https://github.com/heytonyne/fasp.git
cd fasp

# Install dependencies
go mod download
cd frontend && npm install && cd ..

# Run in development mode (with hot reload)
wails dev

# Build for your platform
wails build
```

### Development Commands

```bash
# Hot reload development server
wails dev

# Build production binary
wails build

# Build for specific platform
wails build -platform darwin/amd64
wails build -platform darwin/arm64
wails build -platform windows/amd64

# Run tests (Go backend)
go test ./...

# Format code
go fmt ./...
make fmt
```

## Project Structure

```
fasp/
├── main.go                    # Wails app entry, API bindings
├── go.mod / go.sum            # Go dependencies
├── wails.json                 # Wails configuration
├── Makefile                   # Build/dev commands
├── internal/
│   ├── model/types.go         # Data models (CaptureResult, Settings)
│   ├── service/
│   │   ├── capture/           # Screenshot (darwin/windows)
│   │   ├── clipboard/         # Copy to clipboard (darwin/windows)
│   │   ├── file/              # Save/open dialogs
│   │   ├── hotkey/            # Global hotkeys (darwin/windows)
│   │   ├── permission/        # OS permissions (darwin/windows)
│   │   ├── settings/          # Config persistence
│   │   ├── startup/           # Run at startup (darwin)
│   │   └── upload/            # Upload service (future)
│   ├── tray/                  # System tray (darwin)
│   └── version/               # Version info
├── frontend/
│   ├── src/
│   │   ├── main.tsx           # React entry
│   │   ├── App.tsx            # Main component
│   │   ├── components/
│   │   │   ├── EditorWindow/  # Canvas editor (1257 LOC)
│   │   │   ├── SettingsWindow/
│   │   │   ├── PermissionWarning/
│   │   │   └── Toast/
│   │   ├── store/
│   │   │   ├── editorStore.ts # Zustand editor state (225 LOC)
│   │   │   └── captureStore.ts
│   │   └── types/index.ts     # TypeScript types
│   ├── package.json           # npm dependencies (Konva, Zustand)
│   └── tsconfig.json
├── build/                     # Icons, platform configs
├── scripts/                   # Build helpers
└── docs/
    ├── project-overview-pdr.md
    ├── code-standards.md
    ├── codebase-summary.md
    ├── system-architecture.md
    └── project-roadmap.md
```

## Tech Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| **Backend** | Go 1.24 | Wails v2.11 for native desktop |
| **Frontend** | React 18 + TypeScript | Vite bundling |
| **Canvas** | Konva.js | Scene-graph based editor |
| **State** | Zustand | Lightweight store management |
| **Hotkeys** | golang.design/x/hotkey | Global hotkey registration |

## Architecture Highlights

- **Backend**: Go services for OS integration (screenshot, clipboard, hotkeys)
- **Frontend**: React components with Konva canvas for editor
- **Communication**: Wails runtime IPC between Go and frontend
- **Platform Abstraction**: Go build tags (`_darwin.go`, `_windows.go`) for OS-specific code
- **State Management**: Zustand stores for editor and capture state

## Development Workflow

### Before Committing
1. Run linter: `go fmt ./...` and frontend linting
2. Write/update tests for Go changes
3. Manual testing: capture → annotate → save flow
4. Update relevant docs in `./docs`

### Build Tags for OS-Specific Code
- Files ending in `_darwin.go` compile only on macOS
- Files ending in `_windows.go` compile only on Windows
- Use conditional compilation for platform logic

### Code Standards
- Go: Idiomatic Go with error handling, max 200 LOC per file
- TypeScript: React hooks, functional components, strict types
- React: Zustand stores, composition over props drilling
- CSS: Component-scoped via CSS modules

## File Formats

**Saved Images**:
- PNG (lossless, recommended)
- JPEG (lossy, smaller files)
- WebP (modern compression)

**Settings** (`~/.fasp/settings.json`):
- Hotkey bindings
- Default save path
- Image format preferences
- Upload provider config

## Troubleshooting

**macOS - "Permission Denied" for Screen Recording**:
- Settings → Privacy & Security → Screen Recording
- Add Fasp to allowed apps

**macOS - Tray icon not visible**:
- Restart the app or log out/in
- Check System Preferences for app visibility

**Windows - Hotkeys don't work**:
- Check keyboard layout (non-US layouts may conflict)
- Try different key combinations in settings
- Ensure app has accessibility permissions

## Contributing

1. Read `./docs/code-standards.md` for conventions
2. Create feature branch: `git checkout -b feature/name`
3. Make changes, commit with clear messages
4. Test thoroughly on target platform
5. Submit PR with description

## License

MIT - See LICENSE file

## Related Documentation

- **[Project Overview & PDR](./docs/project-overview-pdr.md)** - Vision, goals, requirements
- **[Code Standards](./docs/code-standards.md)** - Go/TypeScript conventions, patterns
- **[System Architecture](./docs/system-architecture.md)** - Services, components, data flow
- **[Development Roadmap](./docs/project-roadmap.md)** - Current status, upcoming phases
- **[Codebase Summary](./docs/codebase-summary.md)** - Detailed component overview
