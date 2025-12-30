# Version Management

## Version Format

Grabix uses calendar versioning with the format: `vYYYY.MM.PATCH`

- `YYYY` - Year (e.g., 2025)
- `MM` - Month (e.g., 12 for December)
- `PATCH` - Patch number (starts at 0, increments for bug fixes)

### Examples
- `v2025.12.0` - First release in December 2025
- `v2025.12.1` - First patch in December 2025
- `v2026.01.0` - First release in January 2026

## How to Update Version

### 1. Update Version File

Edit `internal/version/version.go`:

```go
var (
    Version = "v2025.12.1"  // Update this line
    BuildTime = ""
    GitCommit = ""
)
```

### 2. Build with Version Info

To include build time and git commit in the version:

```bash
# Get current git commit
GIT_COMMIT=$(git rev-parse HEAD)

# Get current timestamp
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')

# Build with ldflags
go build -ldflags "\
  -X 'github.com/heytonyne/grabix/internal/version.Version=v2025.12.0' \
  -X 'github.com/heytonyne/grabix/internal/version.BuildTime=${BUILD_TIME}' \
  -X 'github.com/heytonyne/grabix/internal/version.GitCommit=${GIT_COMMIT}' \
" -o bin/grabix

# Or use wails build
wails build -ldflags "\
  -X 'github.com/heytonyne/grabix/internal/version.Version=v2025.12.0' \
  -X 'github.com/heytonyne/grabix/internal/version.BuildTime=${BUILD_TIME}' \
  -X 'github.com/heytonyne/grabix/internal/version.GitCommit=${GIT_COMMIT}' \
"
```

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
file build/bin/grabix.app/Contents/MacOS/grabix
```

Output examples:
- ARM64: `Mach-O 64-bit executable arm64`
- AMD64: `Mach-O 64-bit executable x86_64`
- Universal: `Mach-O universal binary with 2 architectures: [x86_64:Mach-O 64-bit executable x86_64] [arm64:Mach-O 64-bit executable arm64]`

## Version Display

Version is displayed in:

1. **Editor Window** - Bottom left corner badge
2. **About Dialog** - macOS menu > Grabix > About Grabix
3. **API** - `GetVersion()` method returns version info

## Release Checklist

- [ ] Update version in `internal/version/version.go`
- [ ] Update CHANGELOG.md with release notes
- [ ] Build with version info using build script
- [ ] Test version display in UI
- [ ] Create git tag: `git tag v2025.12.0`
- [ ] Push tag: `git push origin v2025.12.0`
- [ ] Create GitHub release with binaries

## Version History

### v2025.12.0 (Current)
- Initial release
- Screenshot capture with annotations
- Crop, rectangle, arrow, text, highlight, blur tools
- Hotkey support (F1-F20, navigation keys)
- Settings management
- System tray integration

