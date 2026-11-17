# Build Directory

Build files and assets for the Wails v3 build. See [BUILD.md](../BUILD.md) for the commands.

* `bin` - output (`fasp.app`, `fasp.exe`, the installer); not committed
* `Taskfile.yml` - shared tasks (frontend, bindings, icons)
* `config.yml` - product name, bundle identifier, version, dev-server settings
* `darwin` - macOS files
* `windows` - Windows files

## macOS

- `Taskfile.yml` - builds the binary (arm64, amd64 or universal) and assembles `bin/fasp.app`.
- `Info.plist` - bundle metadata. The identifier `com.wails.fasp` must not change: macOS ties the Screen
  Recording permission to it.
- `Info.dev.plist` - the same, for `make dev`.
- `entitlements.plist` - signed in by `scripts/sign-macos.sh` (hardened runtime, microphone).
- `icons.icns` - app icon, made from `appicon.png`.

## Windows

- `Taskfile.yml` - builds `fasp.exe` and the NSIS installer.
- `icon.ico`, `info.json`, `wails.exe.manifest` - embedded into the exe as a `.syso` resource.
- `nsis/project.nsi`, `nsis/wails_tools.nsh` - the installer. It installs WebView2 when it is missing.

`scripts/stamp-version.sh` writes the release version into the plists, `info.json`, the manifest,
`wails_tools.nsh` and `config.yml`.
