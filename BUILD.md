# Build Instructions

## Prerequisites

- Go 1.21 or later
- Node.js 16 or later
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Xcode Command Line Tools (macOS): `xcode-select --install`

## Quick Build

### Universal Binary (Recommended)

Build for both ARM64 (Apple Silicon) and AMD64 (Intel):

```bash
./scripts/build.sh
```

Output: `build/bin/grabix.app`

### Platform-Specific Builds

**ARM64 only (Apple Silicon M1/M2/M3)**:
```bash
./scripts/build-arm64.sh
```

**AMD64 only (Intel Mac)**:
```bash
./scripts/build-amd64.sh
```

**Universal Binary**:
```bash
./scripts/build-universal.sh
```

## Development Build

For development with hot reload:

```bash
wails dev
```

## Manual Build

If you need to build manually:

```bash
# Get version info
VERSION=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')
GIT_COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')

# Build
wails build -platform darwin/universal -ldflags "\
  -X 'github.com/heytonyne/grabix/internal/version.Version=${VERSION}' \
  -X 'github.com/heytonyne/grabix/internal/version.BuildTime=${BUILD_TIME}' \
  -X 'github.com/heytonyne/grabix/internal/version.GitCommit=${GIT_COMMIT}' \
"
```

## Verify Build

Check the architecture of the built binary:

```bash
file build/bin/grabix.app/Contents/MacOS/grabix
```

Expected outputs:

- **Universal**: `Mach-O universal binary with 2 architectures: [x86_64] [arm64]`
- **ARM64**: `Mach-O 64-bit executable arm64`
- **AMD64**: `Mach-O 64-bit executable x86_64`

## Build Options

### Platform Options

- `darwin/arm64` - macOS ARM64 (Apple Silicon)
- `darwin/amd64` - macOS AMD64 (Intel)
- `darwin/universal` - Universal Binary (both architectures)

### Build Flags

- `-ldflags` - Set version information at build time
- `-platform` - Target platform
- `-clean` - Clean build directory before building
- `-debug` - Build with debug symbols

### Examples

**Clean build**:
```bash
wails build -clean
```

**Debug build**:
```bash
wails build -debug
```

**Production build with all flags**:
```bash
wails build -clean -platform darwin/universal -ldflags "-X 'github.com/heytonyne/grabix/internal/version.Version=v2025.12.0'"
```

## Troubleshooting

### Build fails with "command not found: wails"

Install Wails CLI:
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Make sure `$GOPATH/bin` is in your PATH:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### Build fails with "xcode-select: error"

Install Xcode Command Line Tools:
```bash
xcode-select --install
```

### Frontend build fails

Clean and rebuild frontend:
```bash
cd frontend
npm install
npm run build
cd ..
wails build
```

### Permission denied when running build script

Make scripts executable:
```bash
chmod +x scripts/*.sh
```

## Distribution

### Create DMG (macOS)

```bash
# Install create-dmg
brew install create-dmg

# Create DMG
create-dmg \
  --volname "Grabix" \
  --window-pos 200 120 \
  --window-size 800 400 \
  --icon-size 100 \
  --icon "grabix.app" 200 190 \
  --hide-extension "grabix.app" \
  --app-drop-link 600 185 \
  "Grabix-${VERSION}.dmg" \
  "build/bin/grabix.app"
```

### Code Signing (macOS)

```bash
# Sign the app
codesign --deep --force --verify --verbose --sign "Developer ID Application: Your Name" build/bin/grabix.app

# Verify signature
codesign --verify --verbose build/bin/grabix.app
```

### Notarization (macOS)

Required for distribution outside the Mac App Store:

```bash
# Create zip for notarization
ditto -c -k --keepParent build/bin/grabix.app grabix.zip

# Submit for notarization
xcrun notarytool submit grabix.zip --apple-id "your@email.com" --team-id "TEAMID" --password "app-specific-password"

# Staple the notarization ticket
xcrun stapler staple build/bin/grabix.app
```

