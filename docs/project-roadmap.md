# Fasp Development Roadmap

**Last Updated**: 2025-02-01
**Current Status**: Phase 1 MVP Complete
**Next Phase**: Phase 2 (Advanced Features)

---

## Executive Summary

Fasp has successfully completed Phase 1 MVP with core screenshot, annotation, save, and hotkey features. Phase 2 focuses on refinement, extensibility, and distribution preparation. Phase 3 targets production hardening and auto-update infrastructure.

**Timeline**:
- Phase 1: ✅ Complete (4 weeks)
- Phase 2: 🔄 In Progress (2 weeks planned)
- Phase 3: 📋 Planned (3 weeks planned)

---

## Phase 1: MVP (COMPLETE) ✅

**Duration**: ~4 weeks
**Status**: Done
**Completion Date**: 2025-01-31

### Delivered Features

#### Core Capture
- ✅ Fullscreen screenshot capture (macOS, Windows)
- ✅ Active display capture (single monitor)
- ✅ Multi-display support with coordinate normalization
- ✅ Region selection with visual feedback (freeze + drag)
- ✅ Client-side image cropping via canvas

#### Annotation & Editing
- ✅ Rectangle tool with configurable stroke/fill
- ✅ Arrow tool with end arrowhead
- ✅ Text tool with font size and color
- ✅ Highlight tool (semi-transparent overlay)
- ✅ Color picker integration
- ✅ Stroke width adjustment (1-10px)
- ✅ Full undo/redo history (Ctrl+Z, Ctrl+Y)
- ✅ Zoom controls (25%-200% with scroll wheel)
- ✅ Pan support (drag to move around canvas)
- ✅ Delete annotation (Del key)

#### Save & Export
- ✅ Save to PNG (lossless)
- ✅ Save to JPEG with quality control (0-100)
- ✅ Save to WebP (modern compression)
- ✅ File dialog with default save path
- ✅ Auto-naming with timestamps
- ✅ Copy to clipboard (macOS pbcopy, Windows SetClipboardData)

#### System Integration
- ✅ Global hotkey registration (Cmd+Shift+S / Ctrl+Shift+S)
- ✅ macOS system tray (menu bar icon)
- ✅ Settings persistence (JSON config)
- ✅ Hotkey callback integration
- ✅ macOS Screen Recording permission check
- ✅ Windows DPI awareness
- ✅ macOS run-at-startup option

#### Infrastructure
- ✅ Wails v2.11 framework setup
- ✅ Go service layer architecture
- ✅ React + TypeScript + Zustand frontend
- ✅ Konva.js canvas integration
- ✅ Platform abstraction (build tags)
- ✅ Error handling and user feedback (Toast)
- ✅ Basic documentation (design.md, technical-plan.md)

### Known Issues (Resolved)
- **macOS Tray Icon Timing** (Fixed in v1.1): NSStatusItem visibility on Big Sur
  - Solution: Deferred creation 300ms + CFRetain
  - Status: ✅ Resolved

### Metrics Achieved
- ✅ Capture latency: <500ms
- ✅ 60 FPS annotation rendering
- ✅ Startup time: <2 seconds
- ✅ Binary size: ~15MB (macOS universal)
- ✅ Memory usage: <50MB
- ✅ Works on macOS 11+, Windows 10+

### Test Coverage
- Backend: ~50% test coverage (service layer)
- Frontend: Manual testing complete
- Cross-platform: Tested on macOS 12+ (Intel/Apple Silicon), Windows 11

---

## Phase 2: Advanced Features (NEXT) 🔄

**Planned Duration**: 2 weeks
**Status**: Not Started
**Target Start**: 2025-02-01
**Target End**: 2025-02-15

### Goals
- Refine core editor (split 1257 LOC component)
- Add hotkey customization UI
- Implement upload provider system
- Enhance keyboard shortcuts
- Improve Windows multi-display support
- Add comprehensive UI testing

### Feature Breakdown

#### A. Editor Refactoring (High Priority)
**Issue**: EditorWindow.tsx is 1257 LOC, too large for maintainability

**Tasks**:
1. Split EditorWindow into sub-components:
   - `Canvas.tsx` (300 LOC) - Konva stage + mouse handlers
   - `Toolbar.tsx` (200 LOC) - Tool selection buttons
   - `ActionBar.tsx` (150 LOC) - Save, Copy, Upload buttons
   - `ZoomBar.tsx` (100 LOC) - Zoom controls
   - `EditorWindow.tsx` (200 LOC) - Layout + state orchestration

2. Extract shared logic to custom hooks:
   - `useCanvasDrawing()` - Mouse events, drawing logic
   - `useImageExport()` - Export to PNG/JPEG/WebP
   - `useZoom()` - Zoom/pan state management

**Success Criteria**:
- All components <300 LOC
- No functionality changes
- Hot reload improved
- Component testing possible

#### B. Hotkey Customization UI (Medium Priority)
**Tasks**:
1. Create SettingsWindow component with tabs:
   - Hotkeys tab (customizable binding input)
   - Format tab (PNG/JPEG/WebP selector)
   - Paths tab (default save location picker)

2. Implement hotkey conflict detection:
   - Check if hotkey already registered
   - Warn user before saving
   - Fallback to default if conflict

3. Add UI validation:
   - Invalid hotkey formats rejected
   - Empty fields prevented
   - Success feedback toast

4. Backend support:
   - Unregister old hotkey
   - Register new hotkey
   - Persist in settings.json
   - Test on both platforms

**Success Criteria**:
- User can rebind hotkeys in UI
- Changes persist after restart
- Conflicts detected and warned
- Works on macOS + Windows

#### C. Upload Provider System (Medium Priority)
**Tasks**:
1. Complete UploadService interface (already defined)

2. Implement providers:
   - **Clipboard** (local): Already done via ClipboardService
   - **HTTP Generic**: POST to custom endpoint
   - **Imgur** (future): Via API
   - **S3** (future): AWS integration

3. Provider selection UI:
   - Dropdown in ActionBar
   - API endpoint input for HTTP provider
   - Token storage (secure)

4. Upload flow:
   - Export canvas as PNG blob
   - Call UploadService.Upload(blob)
   - Show progress indicator
   - Copy URL to clipboard on success

**Success Criteria**:
- HTTP generic upload working
- Provider can be selected and configured
- Upload completes in <3s
- URL copied to clipboard

#### D. Keyboard Shortcuts in Editor (Low Priority)
**Tasks**:
1. Document shortcuts:
   - Ctrl+Z / Cmd+Z: Undo
   - Ctrl+Y / Cmd+Y: Redo
   - Ctrl+S / Cmd+S: Save
   - Ctrl+C / Cmd+C: Copy to clipboard
   - Ctrl+A: Select all annotations
   - Del: Delete selected
   - Numbers (1-4): Quick tool selection

2. Implement handlers:
   - Hook into window keydown events
   - Prevent browser defaults (Ctrl+S)
   - Update UI for visual feedback

3. Add help overlay (Ctrl+H):
   - Show keyboard shortcut reference
   - Display in modal

**Success Criteria**:
- All listed shortcuts work
- Shortcuts documented in UI
- No conflicts with browser shortcuts

#### E. Windows Multi-Display Fixes (Low Priority)
**Tasks**:
1. DPI scaling normalization:
   - Test on multi-monitor setup with different DPI
   - Fix coordinate offsets (100% vs 125% vs 150%)
   - Ensure screenshot bounds correct

2. Monitor detection:
   - Enumerate all monitors
   - Get DPI for each
   - Apply scaling to coordinates

**Success Criteria**:
- Works on dual-monitor setup (mixed DPI)
- Screenshot bounds accurate
- Region selection aligns with display

#### F. Component Testing (Medium Priority)
**Tasks**:
1. Setup Jest + React Testing Library

2. Write tests for:
   - EditorWindow (rendering, tool selection)
   - Toolbar (button clicks)
   - Toast (notifications)
   - SettingsWindow (input validation)
   - Custom hooks (useEditorUndo, useImage)

3. Coverage target: ≥60% for React code

**Success Criteria**:
- All components have unit tests
- Integration test for capture → edit → save
- CI runs tests on PR

### Timeline

| Week | Task | Owner | Status |
|------|------|-------|--------|
| Week 1 | Editor refactoring + hotkey UI | Dev | 📋 Planned |
| Week 1 | Upload service implementation | Dev | 📋 Planned |
| Week 2 | Keyboard shortcuts + Windows fixes | Dev | 📋 Planned |
| Week 2 | Component testing suite | QA | 📋 Planned |

### Success Metrics
- ✅ EditorWindow split into 4 sub-components
- ✅ Hotkey customization working on both platforms
- ✅ Upload to HTTP endpoint functional
- ✅ ≥60% test coverage for React
- ✅ Zero regressions in Phase 1 features

---

## Phase 3: Polish & Distribution (FUTURE) 📋

**Planned Duration**: 3 weeks
**Status**: Not Started
**Target Start**: 2025-02-16

### Goals
- Production-ready build and distribution
- Code signing and notarization
- Auto-update infrastructure
- Performance optimization
- Comprehensive documentation

### Feature Breakdown

#### A. Code Signing & Notarization (High Priority)
**macOS**:
1. Acquire Apple Developer ID certificate ($99/year)
2. Code sign app bundle: `codesign --deep -s "Developer ID"`
3. Create notarization ticket: `altool --notarize-app`
4. Staple notarization: `stapler staple fasp.app`

**Windows**:
1. Optional: Acquire Authenticode certificate
2. Sign executable: `signtool sign /f cert.pfx fasp.exe`

**Success Criteria**:
- macOS: Gatekeeper accepts app without warnings
- Windows: SmartScreen doesn't block (optional signed)
- No notarization hold-ups

#### B. Installer Creation (High Priority)
**macOS**:
1. Create .dmg disk image:
   - App bundle + Applications shortcut
   - Custom background image
   - Drag-to-install instructions

**Windows**:
1. Choose installer framework:
   - NSIS (free, simple)
   - WiX Toolset (advanced, MSI format)
2. Create installer with:
   - Installation path selection
   - Start menu shortcuts
   - Auto-launch option

**Success Criteria**:
- Installer works on fresh system
- Uninstaller removes all files
- Shortcuts functional after install

#### C. Auto-Update System (Medium Priority)
**Options**:
1. **Built-in Wails**: v2.11 has auto-update support
2. **Sparkle** (macOS): Lightweight update framework
3. **GitHub Releases**: Check latest release tag

**Tasks**:
1. Implement update checker (background)
2. Prompt user if new version available
3. Download and install silently
4. Restart app after update
5. Rollback on failure

**Success Criteria**:
- Check for updates in background
- User prompted for install
- Auto-restart with new version
- No data loss

#### D. Performance Optimization (Low Priority)
**Tasks**:
1. Profile memory usage (heap snapshots)
2. Optimize image rendering:
   - Lazy load large images
   - Canvas resolution limits
3. Reduce binary size:
   - Tree-shake unused deps
   - Minify frontend assets
4. Startup time optimization:
   - Lazy init services
   - Parallel startup operations

**Targets**:
- Memory: <40MB idle
- Binary: <20MB total
- Startup: <1.5s

#### E. Documentation (High Priority)
**Tasks**:
1. User documentation:
   - Installation guide (per OS)
   - Quick start tutorial
   - Hotkey reference
   - Troubleshooting FAQ

2. Developer documentation:
   - Architecture overview
   - API reference (Go services)
   - Development setup
   - Contributing guide

3. Update existing docs:
   - Keep design.md in sync
   - Update technical-plan.md
   - Add deployment guide

**Success Criteria**:
- New users can install and use without friction
- Developers can fork and extend
- Troubleshooting covers common issues

#### F. CI/CD Pipeline (Medium Priority)
**Tasks**:
1. GitHub Actions workflows:
   - Build on every PR (macOS + Windows)
   - Run tests (Go + Jest)
   - Linting (golangci-lint + ESLint)
   - Code coverage reporting

2. Release automation:
   - Build all platforms on tag push
   - Create GitHub Release with binaries
   - Auto-generate changelog
   - Create .dmg and .exe installers

**Success Criteria**:
- Tests run automatically on PR
- Release builds all platforms
- Binaries uploaded to release page

### Timeline

| Week | Task | Owner | Status |
|------|------|-------|--------|
| Week 1 | Code signing + macOS notarization | Ops | 📋 Planned |
| Week 1 | Installer creation (.dmg, .exe) | Ops | 📋 Planned |
| Week 2 | Auto-update system | Dev | 📋 Planned |
| Week 2 | Performance optimization | Dev | 📋 Planned |
| Week 3 | Documentation finalization | Docs | 📋 Planned |
| Week 3 | CI/CD pipeline setup | DevOps | 📋 Planned |

### Success Metrics
- ✅ macOS app passes Gatekeeper + notarization
- ✅ Installers work on fresh system
- ✅ Auto-update mechanism functional
- ✅ Binary <20MB, memory <40MB
- ✅ Complete user + developer documentation
- ✅ All tests pass in CI/CD

---

## Future Roadmap (Phase 4+)

### Cloud Integration
- [ ] Imgur uploader (auto URL → clipboard)
- [ ] AWS S3 integration
- [ ] Dropbox sync
- [ ] Google Drive upload
- [ ] Custom webhook support

### Advanced Editing
- [ ] Color picker UI (hex/RGB input)
- [ ] Text formatting (bold, italic, size presets)
- [ ] Blur/redact tool (pixelate sensitive areas)
- [ ] Multiple colors per annotation
- [ ] Annotation templates/presets

### Collaboration
- [ ] Shareable links with expiry
- [ ] Comment/annotation on shared images
- [ ] Collaborator permissions
- [ ] Activity feed

### Mobile
- [ ] iOS app (SwiftUI)
- [ ] Android app (Compose)
- [ ] Shared clipboard sync
- [ ] Mobile-to-desktop annotation

### AI Features
- [ ] OCR (extract text from screenshot)
- [ ] Auto-format detection (UI element highlighting)
- [ ] Smart tagging/organization
- [ ] Search by content

---

## Blocked Issues & Risks

### Known Limitations
| Item | Status | Impact | Plan |
|------|--------|--------|------|
| EditorWindow too large (1257 LOC) | 🔴 Blocked | Code maintenance | Phase 2 refactoring |
| No React component tests | 🔴 Blocked | Regression risk | Phase 2 testing |
| Upload service unimplemented | 🟡 Partial | Cloud sharing | Phase 2 feature |
| Windows DPI scaling untested | 🟡 Partial | Multi-monitor bugs | Phase 2 testing |
| No auto-update | 🟡 Partial | Manual updates | Phase 3 feature |

### Timeline Risks
| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|-----------|
| macOS notarization delays | Low | 2-3 days | Start early, use sandbox mode |
| Hotkey conflict detection complexity | Low | 1 day | Implement in Phase 2 |
| Windows DPI multi-monitor issues | Medium | 2 days | Test early in Phase 2 |
| Component refactoring regressions | Medium | 1 day | Comprehensive testing |

### Dependency Risks
- **Wails updates**: v3 coming with breaking changes → monitor releases
- **React updates**: Major version changes → test carefully
- **Konva performance**: Large images may lag → profile early

---

## Dependencies & Blockers

### Phase 2 Prerequisites
- ✅ Phase 1 MVP complete
- ✅ Codebase stable (no critical bugs)
- ✅ Test framework ready (Jest setup)

### Phase 3 Prerequisites
- ✅ Phase 2 complete
- ✅ No blockers from Phase 2
- ✅ Installer tools available (NSIS, .dmg tools)
- ✅ Apple Developer account (for signing)

---

## Metrics & Success Criteria

### Overall Quality Gates
- [ ] Unit test coverage: ≥60%
- [ ] Integration test coverage: ≥40%
- [ ] Zero critical bugs in release
- [ ] All platforms tested
- [ ] Documentation complete
- [ ] <1% crash rate (in field)

### Performance Targets
| Metric | Target | Phase |
|--------|--------|-------|
| Capture latency | <500ms | 1 ✅ |
| Startup time | <2s | 1 ✅ |
| Save latency | <1s | 1 ✅ |
| Memory (idle) | <40MB | 3 |
| Binary size | <20MB | 3 |
| Canvas FPS | 60 FPS | 1 ✅ |

---

## Resource Planning

### Phase 2 (2 weeks)
- **Dev**: 1-2 engineers (full-time)
- **QA**: 1 tester (part-time)
- **Effort**: ~80 hours total

### Phase 3 (3 weeks)
- **Dev**: 1-2 engineers (full-time)
- **Ops**: 1 DevOps engineer (part-time)
- **Docs**: 1 technical writer (part-time)
- **Effort**: ~150 hours total

---

## Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-02-01 | Team | Initial roadmap, Phase 1 complete |

---

**Current Status**: MVP Phase Complete, Ready for Phase 2
**Next Milestone**: 2025-02-15 (Phase 2 completion target)
**Quarterly Goals**: Complete Phase 2 + begin Phase 3 prep
