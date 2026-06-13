# Session Summary - 2025-12-20

## Overview
This session focused on implementing UI improvements, new annotation features, and system tray integration for Fasp screenshot application.

## Features Implemented

### 1. Canvas Expansion & Pan Support
**Problem**: Main image display was too small, wasting layout space.

**Solution**:
- Increased canvas size from `window.innerWidth - 100` to `window.innerWidth - 40`
- Increased canvas height from `window.innerHeight - 200` to `window.innerHeight - 140`
- Added violet border around image: `stroke="rgba(139, 92, 246, 0.4)"`
- Implemented drag-to-pan when zoomed in (zoom > 1)
- Added cursor changes: `grab` → `grabbing` during pan
- State management: `stagePos: { x, y }`, `isPanning: boolean`

**Files Modified**:
- `frontend/src/components/EditorWindow/EditorWindow.tsx`
- `frontend/src/components/EditorWindow/EditorWindow.css`

### 2. UI Compaction & Icon-Only Toolbar
**Problem**: Buttons and toolbar taking too much space from main image.

**Solution**:
- **Toolbar**: Icon-only buttons, removed text labels
  - Padding: `10px 16px` → `6px 12px`
  - Button size: `32px × 32px`
  - Icon size: `16px`
  - Gap: `8px` → `4px`
- **Tool Icons**: Changed to Unicode symbols
  - Select: `↖`, Crop: `✂`, Rectangle: `▭`, Arrow: `→`, Text: `T`, Highlight: `◧`
- **Color Picker**: Smaller buttons (`24px` → `20px`)
- **Action Bar**: Reduced padding and font sizes
- **Result**: Saved ~35px vertical space for main image

**Files Modified**:
- `frontend/src/components/EditorWindow/EditorWindow.css`
- `frontend/src/components/EditorWindow/Toolbar.tsx`

### 3. Blur Annotation Tool
**Problem**: Need to blur/obscure sensitive information in screenshots.

**Solution**:
- Added `'blur'` to `AnnotationTool` type
- Toolbar icon: `◎` (circle with dot)
- Implementation using Konva's Blur filter:
  - `fillPatternImage`: Use original image as pattern
  - `fillPatternX/Y`: Offset to align with blur region
  - `node.cache()` + `node.filters([Konva.Filters.Blur])`
  - `node.blurRadius(20)`: 20px blur intensity
- Supports resize and drag like other annotations

**Files Modified**:
- `frontend/src/types/index.ts`
- `frontend/src/components/EditorWindow/EditorWindow.tsx`
- `frontend/src/components/EditorWindow/Toolbar.tsx`

### 4. Window Management & Screenshot Flow Fix
**Problem**: App was showing window BEFORE screenshot, capturing its own window.

**Solution**:
- Fixed capture flow:
  1. `WindowHide()` - Hide window first
  2. `await new Promise(resolve => setTimeout(resolve, 200))` - Wait 200ms
  3. `CaptureActiveDisplay()` - Capture screenshot
  4. `WindowShow()` + `WindowUnminimise()` - Show window with captured image
- Error handling: Show window even if capture fails

**Files Modified**:
- `frontend/src/App.tsx`

### 5. Toast Notifications
**Problem**: Alert dialogs blocking UI, no visual feedback for actions.

**Solution**:
- Created Toast component with glassmorphism design
- Features:
  - Types: `success` (green), `error` (red), `info` (blue)
  - Auto-dismiss after 3 seconds (configurable)
  - Slide-in animation from right
  - Position: Fixed top-right corner
- Replaced all `alert()` calls with toast notifications
- Messages: "Copied to clipboard!", "Saved to {filename}", etc.

**Files Created**:
- `frontend/src/components/Toast/Toast.tsx`
- `frontend/src/components/Toast/Toast.css`

**Files Modified**:
- `frontend/src/components/EditorWindow/ActionBar.tsx`

### 6. Quick Save Feature
**Problem**: Need fast save without opening dialog every time.

**Solution**:
- New button: `⚡ Quick Save` (primary button)
- Renamed existing save: `💾 Save As` (secondary button)
- Quick Save workflow:
  - Get `default_save_path` from settings
  - Fallback: `/Users/Shared/Fasp`
  - Auto-generate filename: `screenshot_YYYY-MM-DD_HH-MM-SS.png`
  - Save directly without dialog
  - Toast: "Saved to {filename}"

**Files Modified**:
- `frontend/src/components/EditorWindow/ActionBar.tsx`

### 7. Auto-Close Window After Actions
**Problem**: Window stays open after copy/save, cluttering workspace.

**Solution**:
- After successful action (Quick Save, Save As, Copy):
  - Show toast notification
  - Wait 1.5 seconds (for toast visibility)
  - `WindowHide()` - Hide window
- App continues running in background
- Can reopen from menu bar

**Files Modified**:
- `frontend/src/components/EditorWindow/ActionBar.tsx`

### 8. Menu Bar Integration
**Problem**: Need system tray/menu bar for background app access.

**Initial Attempt**: Used `github.com/getlantern/systray`
- **Failed**: Duplicate symbol `AppDelegate` conflict with Wails

**Final Solution**: Wails native menu API
- **File Menu**:
  - Capture Screenshot - Trigger capture
  - Quit (Cmd+Q) - Quit app
- **Window Menu**:
  - Show Window - Show main window
  - Hide Window - Hide window
- **macOS Integration**:
  - `TitleBarDefault()` - Keep titlebar for window dragging
  - Custom About dialog
  - Native menu bar appearance

**Files Modified**:
- `main.go` - Added `buildMenu()` function, menu integration

**Dependencies Removed**:
- `github.com/getlantern/systray` (caused conflicts)

### 9. App Startup Behavior
**Configuration**:
- `StartHidden: true` - App starts hidden, only menu bar visible
- `HideWindowOnClose: true` - Close button hides instead of quitting
- Window only shows when:
  - Hotkey pressed (after screenshot capture)
  - Menu → Window → Show Window
  - Menu → File → Capture Screenshot

**Files Modified**:
- `main.go`

## Technical Challenges & Solutions

### Challenge 1: Systray AppDelegate Conflict
**Problem**: `github.com/getlantern/systray` creates its own `AppDelegate`, conflicting with Wails' `AppDelegate`.

**Error**:
```
duplicate symbol '_OBJC_METACLASS_$_AppDelegate'
duplicate symbol '_OBJC_CLASS_$_AppDelegate'
ld: 2 duplicate symbols
```

**Solution**: Switched to Wails native menu API instead of external systray library.

### Challenge 2: Screenshot Capturing Own Window
**Problem**: Window was shown before screenshot, resulting in capturing the app itself.

**Solution**: Reversed flow - hide window first, wait 200ms, capture, then show window.

### Challenge 3: Process.env Not Available in Browser
**Problem**: TypeScript error when using `process.env.HOME` in frontend.

**Solution**: Use fallback path `/Users/Shared/Fasp` instead of environment variables.

### Challenge 4: Title Bar Missing
**Problem**: `TitleBarHiddenInset()` removed ability to drag window.

**Solution**: Changed to `TitleBarDefault()` to keep standard macOS titlebar.

## Complete Workflow

### User Journey
1. **Start App**
   - App starts hidden
   - Menu bar shows "Fasp" with File/Window menus
   - Dock icon visible (macOS)

2. **Capture Screenshot**
   - Press `Alt+F11` (or menu → Capture Screenshot)
   - Window hides (200ms)
   - Screenshot captured
   - Window appears with captured image

3. **Annotate**
   - Use tools: Rectangle, Arrow, Text, Highlight, Blur, Crop
   - Zoom in/out with slider
   - Pan image when zoomed (drag with mouse)
   - Undo/Redo changes

4. **Save or Copy**
   - **Quick Save**: Click `⚡ Quick Save` → Auto-save to default folder → Toast → Window hides
   - **Save As**: Click `💾 Save As` → Choose location → Save → Toast → Window hides
   - **Copy**: Click `📋 Copy` → Copy to clipboard → Toast → Window hides

5. **Reopen Window**
   - Menu bar → Window → Show Window
   - Or press hotkey again for new screenshot

6. **Quit**
   - Menu bar → File → Quit
   - Or press `Cmd+Q`

## Files Changed Summary

### Frontend
- `frontend/src/App.tsx` - Window management, capture flow
- `frontend/src/components/EditorWindow/EditorWindow.tsx` - Canvas size, pan, blur tool
- `frontend/src/components/EditorWindow/EditorWindow.css` - UI compaction
- `frontend/src/components/EditorWindow/Toolbar.tsx` - Icon-only toolbar
- `frontend/src/components/EditorWindow/ActionBar.tsx` - Quick save, auto-close, toast
- `frontend/src/components/Toast/Toast.tsx` - Toast component (NEW)
- `frontend/src/components/Toast/Toast.css` - Toast styles (NEW)
- `frontend/src/types/index.ts` - Added 'blur' annotation type

### Backend
- `main.go` - Menu bar, app options, startup behavior
- `internal/tray/tray_darwin.go` - Native tray icon (CGO) (NEW - WIP)

### Dependencies
- **Removed**: `github.com/getlantern/systray` (conflict with Wails)
- **Added**: Wails menu packages (`menu`, `menu/keys`, `options/mac`)

## Known Issues & Future Work

### Current Issues
1. **Tray Icon**: Native tray icon implementation incomplete (CGO approach started)
2. **Windows Support**: Menu bar implementation is macOS-specific

### Future Enhancements
1. Complete native tray icon with NSStatusBar (macOS)
2. Windows tray icon implementation
3. Settings window for configuring default save path
4. Hotkey customization UI
5. Upload to cloud providers (planned in original design)

## Build & Test

### Development
```bash
wails dev
```

### Production Build
```bash
wails build
```

### Frontend Only
```bash
cd frontend
npm run build
```

## Key Learnings

1. **Wails Menu API** is the correct way to add menus, not external systray libraries
2. **Window timing** is critical for screenshot apps - hide before capture
3. **Toast notifications** provide better UX than blocking alerts
4. **Auto-hide window** keeps workspace clean while app runs in background
5. **Icon-only toolbar** saves significant screen space for main content

## Session Statistics

- **Duration**: ~2 hours
- **Features Implemented**: 9 major features
- **Files Modified**: 8 files
- **Files Created**: 3 files
- **Dependencies Changed**: Removed 1, added Wails menu packages
- **Build Status**: ✅ Successful (with warnings about private APIs for dev build)

---

**Session Date**: 2025-12-20
**Developer**: Xếp (Principal Golang Developer)
**AI Assistant**: Augment Agent (Claude Sonnet 4.5)


