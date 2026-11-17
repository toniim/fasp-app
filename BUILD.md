# Build Instructions

Fasp is built with [Wails v3](https://v3.wails.io). The build runs through the `wails3` CLI and the
Taskfiles in `Taskfile.yml` and `build/`. The `make` targets wrap them and stamp the version.

## Prerequisites

- Go 1.26 (the toolchain pinned in `go.mod`; Go 1.27 is not used)
- Node.js 18 or later
- Wails v3 CLI at the version in `go.mod`: `make install`, or
  `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`
- macOS: Xcode Command Line Tools (`xcode-select --install`)
- Windows installer: [NSIS](https://nsis.sourceforge.io) (`makensis` on the PATH)

Make sure `$(go env GOPATH)/bin` is on your PATH so `wails3` is found.

## Quick Build

```bash
make build-darwin-signed   # macOS app (arm64), signed with the "Fasp Dev" certificate
make build-darwin          # macOS app (arm64), ad-hoc signed
make build-windows         # fasp.exe and fasp-amd64-installer.exe
```

Output goes to `build/bin/`: `fasp.app`, `fasp.exe`, `fasp-amd64-installer.exe`.

### Universal macOS binary (Apple Silicon + Intel)

```bash
./scripts/build.sh                # darwin/universal by default
./scripts/build.sh darwin/arm64   # or darwin/amd64
```

or `wails3 task darwin:package:universal`. `./scripts/package-macos.sh` then signs and zips it.

## Development

```bash
make dev    # wails3 dev: Vite hot reload plus the Go app
```

## Manual Build

`FASP_LDFLAGS` carries the `-X` flags that stamp `internal/version`; the Taskfile adds it to `go build`.

```bash
export FASP_LDFLAGS="\
  -X 'github.com/heytonyne/fasp/internal/version.Version=$(grep 'Version = ' internal/version/version.go | sed 's/.*"\(.*\)".*/\1/')' \
  -X 'github.com/heytonyne/fasp/internal/version.BuildTime=$(date -u '+%Y-%m-%d %H:%M:%S')' \
  -X 'github.com/heytonyne/fasp/internal/version.GitCommit=$(git rev-parse HEAD)'"

export GOTOOLCHAIN=$(sed -n 's/^toolchain //p' go.mod)
wails3 package GOOS=darwin GOARCH=arm64     # build/bin/fasp.app
wails3 package GOOS=windows GOARCH=amd64    # build/bin/fasp.exe + installer
wails3 build GOOS=windows GOARCH=amd64      # build/bin/fasp.exe only
```

The Windows app has no C code, so `fasp.exe` also cross-compiles from macOS or Linux. The installer needs `makensis`.

## Bindings

The frontend calls the Go `App` service through generated TypeScript in `frontend/bindings/`. The build
regenerates it; to do it by hand after changing the exported methods or their types of `App`:

```bash
wails3 generate bindings -clean=true -ts -i -d frontend/bindings
```

Commit the result.

## Verify Build

```bash
file build/bin/fasp.app/Contents/MacOS/fasp
codesign --verify --deep --strict build/bin/fasp.app
```

A universal build reports `Mach-O universal binary with 2 architectures: [x86_64] [arm64]`.

## Build files

| Path | What it is |
|---|---|
| `Taskfile.yml`, `build/Taskfile.yml`, `build/darwin/Taskfile.yml`, `build/windows/Taskfile.yml` | Build tasks |
| `build/config.yml` | Product name, bundle id `com.wails.fasp`, version, dev-server settings |
| `build/darwin/Info.plist` | macOS bundle metadata (also the microphone usage text) |
| `build/darwin/entitlements.plist` | Entitlements `scripts/sign-macos.sh` signs with |
| `build/windows/` | Icon, manifest, version info and the NSIS installer script |

Do not run `wails3 update build-assets`: it overwrites `Info.plist` and the installer files with the
defaults. Icons are regenerated with `wails3 task common:generate:icons`.

## Troubleshooting

- **`command not found: wails3`**: run `make install` and add `$(go env GOPATH)/bin` to the PATH.
- **`xcode-select: error`**: `xcode-select --install`.
- **`make` says the wails3 CLI differs from go.mod**: run `make install`.
- **Frontend build fails**: `cd frontend && npm install && npm run build`.

## Distribution

Releases are built by the Release workflow (see the README, "Releasing"). The macOS app is signed with the
"Fasp Dev" certificate by `scripts/sign-macos.sh`; it is not notarized.
