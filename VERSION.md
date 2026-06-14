# Version Management

## Version Format

`internal/version/version.go` is the **single source of truth** for the version.
Fasp uses calendar versioning with the format: `vYY.M.PATCH`

- `YY` - Year, 2 digits (e.g., 26 for 2026)
- `M` - Month, no leading zero (e.g., 1 for January, 12 for December)
- `PATCH` - Patch number (starts at 0, increments for bug fixes)

### Examples
- `v26.1.0` - First release in January 2026
- `v26.1.1` - First patch in January 2026
- `v26.12.0` - First release in December 2026

The build scripts (`scripts/build.sh`) and the Makefile build targets read this
value from `version.go` and stamp it into the binary via `-ldflags`, so the value
in `version.go` is the only thing you edit.

## How to Update Version

### 1. Update Version File

Edit `internal/version/version.go`:

```go
var (
    Version = "v26.1.1"  // Update this line only
    BuildTime = ""
    GitCommit = ""
)
```

### 2. Build with Version Info

The Makefile reads the version from `version.go` and injects build time + git
commit automatically:

```bash
make build           # current platform
make build-windows   # windows/amd64
make build-darwin    # darwin/arm64
```

Equivalent manual command (what the Makefile / build.sh run):

```bash
VERSION=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
wails build -ldflags "\
  -X 'github.com/heytonyne/fasp/internal/version.Version=${VERSION}' \
  -X 'github.com/heytonyne/fasp/internal/version.BuildTime=$(date -u '+%Y-%m-%d %H:%M:%S')' \
  -X 'github.com/heytonyne/fasp/internal/version.GitCommit=$(git rev-parse HEAD)' \
"
```

### 3. Tag the release (follows version.go)

```bash
./scripts/release.sh   # or: make release
```

Both read the version from `version.go`, tag the current commit, and push the
tag. They refuse to run if the working tree is dirty or the tag already exists —
so the flow is: bump `version.go` → commit → `./scripts/release.sh`.
(Use the script on Windows/Git Bash where `make` isn't installed.)

### 3. Build for Different Platforms

**Universal Binary (ARM64 + AMD64)** - Recommended for distribution:
```bash
./scripts/build.sh
# or
./scripts/build-universal.sh
```

**ARM64 only (Apple Silicon)**:
```bash
./scripts/build-arm64.sh
# or
./scripts/build.sh darwin/arm64
```

**AMD64 only (Intel Mac)**:
```bash
./scripts/build-amd64.sh
# or
./scripts/build.sh darwin/amd64
```

**Check architecture**:
```bash
file build/bin/fasp.app/Contents/MacOS/fasp
```

Output examples:
- ARM64: `Mach-O 64-bit executable arm64`
- AMD64: `Mach-O 64-bit executable x86_64`
- Universal: `Mach-O universal binary with 2 architectures: [x86_64:Mach-O 64-bit executable x86_64] [arm64:Mach-O 64-bit executable arm64]`

## Version Display

Version is displayed in:

1. **Editor Window** - Bottom left corner badge
2. **About Dialog** - macOS menu > Fasp > About Fasp
3. **API** - `GetVersion()` method returns version info

## Release Checklist

- [ ] Bump version in `internal/version/version.go` (e.g. `v26.1.1`)
- [ ] Commit the bump
- [ ] Build & smoke-test (`make build`), verify the version badge in the UI
- [ ] `make release` (tags the commit with the version.go value and pushes it)
- [ ] Create the GitHub release for that tag and attach the built binaries

## Version History

### v26.1.0 (Current)
- API-key uploads to fasp, configurable server URL
- Editor: multi-select, text box tool, easier selection/resize/rotate
- Remembers editor window state

### v25.12.0
- Initial release
- Screenshot capture with annotations
- Crop, rectangle, arrow, text, highlight, blur tools
- Hotkey support (F1-F20, navigation keys)
- Settings management
- System tray integration

