# Code Standards & Conventions - Grabix

## 1. Go Backend Standards

### File Organization
- **Max file size**: 200 lines (split if exceeds)
- **Naming**: snake_case for files (`capture_darwin.go`, `hotkey_windows.go`)
- **Build tags**: OS-specific code in separate files with tags:
  ```go
  // +build darwin
  // or
  //go:build windows
  ```
- **Imports**: Organize in groups (stdlib, external, internal)
  ```go
  import (
      "fmt"
      "os"

      "github.com/wailsapp/wails/v2"
      "golang.design/x/hotkey"

      "github.com/heytonyne/grabix/internal/model"
  )
  ```

### Naming Conventions
- **Packages**: lowercase, single word (`capture`, `hotkey`, `file`)
- **Types**: PascalCase (`CaptureService`, `HotkeyEvent`)
- **Functions**: PascalCase for exported, camelCase for private
  ```go
  func (s *CaptureService) CaptureFullscreen() ([]byte, error)
  func (s *captureService) encodeImage(img image.Image) ([]byte, error)
  ```
- **Constants**: ALL_CAPS with underscore
  ```go
  const (
      MAX_IMAGE_WIDTH = 4096
      DEFAULT_FORMAT = "png"
  )
  ```
- **Variables**: camelCase
  ```go
  var imageBuffer []byte
  ```

### Interface Design
- Small, focused interfaces (1-3 methods where possible)
- Name ending with "er" or "Service": `CaptureService`, `FileWriter`
- Always document contract with comments
  ```go
  // CaptureService handles screenshot operations
  type CaptureService interface {
      // CaptureFullscreen captures the entire screen
      CaptureFullscreen(ctx context.Context) ([]byte, error)
  }
  ```

### Error Handling
- **Always check errors** - no silent failures
- Use `fmt.Errorf()` with context:
  ```go
  if err != nil {
      return fmt.Errorf("failed to capture screenshot: %w", err)
  }
  ```
- Custom error types for specific cases:
  ```go
  type PermissionError struct {
      reason string
  }

  func (e *PermissionError) Error() string {
      return fmt.Sprintf("permission denied: %s", e.reason)
  }
  ```
- Wrap errors with `%w` verb for error chain inspection

### Testing
- **Test files**: `*_test.go` in same package
- **Test naming**: `TestFunctionName(t *testing.T)`
- **Mocking**: Use `mockery` for interface mocks
- **Coverage target**: ≥50% for business logic
  ```bash
  go test ./... -cover
  mockery --all --output=mocks
  ```

### Code Style
- **Formatting**: `go fmt` (standard Go formatter)
- **Linting**: `golangci-lint run ./...`
- **Line length**: Hard limit 120 chars (readability)
- **Indentation**: Tabs (Go standard)
- **Comments**:
  - Exported functions: "// FunctionName does X"
  - Inline comments for complex logic only
  - No commented-out code blocks

### Concurrency
- Use contexts for cancellation:
  ```go
  func (s *Service) Capture(ctx context.Context) error
  ```
- Prefer channels over shared memory
- Use `sync.Once` for lazy initialization
- Document goroutine lifecycle and cleanup

### Platform-Specific Code
Isolate OS logic in separate files:
```
service/
├── service.go           # Interface
├── service_darwin.go    # macOS implementation
└── service_windows.go   # Windows implementation
```

Then use build tags or feature detection:
```go
// In service.go
var impl Service = newPlatformService()

func newPlatformService() Service {
    // Will use platform-specific implementation
    return newDarwinService() // on macOS
}
```

---

## 2. TypeScript/React Frontend Standards

### File Organization
- **Max file size**: 200 lines (split large components)
- **Naming**: kebab-case for files (`editor-window.tsx`, `use-hotkey.ts`)
- **Directory structure**: Feature-based (not layer-based)
  ```
  components/
  ├── EditorWindow/
  │   ├── EditorWindow.tsx
  │   ├── Toolbar.tsx
  │   ├── Canvas.tsx
  │   └── EditorWindow.module.css
  ├── SettingsWindow/
  └── Toast/
  ```

### Type Annotations
- **Always use strict types** (no `any`)
- Export types for public APIs:
  ```typescript
  // types.ts
  export type AnnotationTool = 'rectangle' | 'arrow' | 'text' | 'highlight'
  export interface Annotation {
      id: string
      type: AnnotationTool
      color: string
  }

  // component.tsx
  interface EditorProps {
      image: string
      onSave: (data: Blob) => Promise<void>
  }
  ```
- Use `unknown` instead of `any` for catch blocks:
  ```typescript
  try {
      // ...
  } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
  }
  ```

### React Component Patterns
- **Functional components only** - no class components
- **Use hooks** - useState, useEffect, useCallback, useMemo
- **Composition over props drilling**:
  ```typescript
  // Bad - prop drilling
  function Parent(props) {
      return <Child color={props.color} size={props.size} />
  }

  // Good - composition
  function Parent({ children, ...theme }) {
      return <ThemeProvider {...theme}>{children}</ThemeProvider>
  }
  ```
- **Custom hooks** for reusable logic:
  ```typescript
  function useKeyDown(key: string, callback: () => void) {
      useEffect(() => {
          const handler = (e: KeyboardEvent) => {
              if (e.key === key) callback()
          }
          window.addEventListener('keydown', handler)
          return () => window.removeEventListener('keydown', handler)
      }, [key, callback])
  }
  ```

### State Management (Zustand)
- **Store structure**: One file per domain
  ```typescript
  // editorStore.ts
  interface EditorState {
      annotations: Annotation[]
      selectedTool: AnnotationTool
      undoStack: Annotation[][]
      redoStack: Annotation[][]
  }

  export const useEditorStore = create<EditorState>((set) => ({
      annotations: [],
      selectedTool: 'rectangle',
      undoStack: [],
      redoStack: [],

      addAnnotation: (anno) => set((state) => ({
          annotations: [...state.annotations, anno],
          redoStack: [] // Clear redo on new action
      })),

      undo: () => set((state) => {
          if (state.undoStack.length === 0) return state
          const prev = state.undoStack[state.undoStack.length - 1]
          return {
              annotations: prev,
              undoStack: state.undoStack.slice(0, -1),
              redoStack: [...state.redoStack, state.annotations]
          }
      })
  }))
  ```
- **No Redux-style selectors** - access state directly
- **Actions as store methods** - keeps logic colocated

### Naming Conventions
- **Components**: PascalCase (`EditorWindow`, `Toast`)
- **Hooks**: camelCase with "use" prefix (`useEditorStore`, `useKeyDown`)
- **Types**: PascalCase ending with "Props" or "State"
  ```typescript
  interface EditorWindowProps {
      imageData: Blob
  }
  ```
- **Constants**: UPPER_SNAKE_CASE
  ```typescript
  const MAX_ZOOM_LEVEL = 5
  const SUPPORTED_FORMATS = ['png', 'jpeg', 'webp'] as const
  ```
- **Variables**: camelCase
  ```typescript
  const [isLoading, setIsLoading] = useState(false)
  ```

### Styling
- **CSS Modules** for component-scoped styles:
  ```typescript
  // EditorWindow.module.css
  .container { ... }
  .toolbar { ... }

  // EditorWindow.tsx
  import styles from './EditorWindow.module.css'
  export function EditorWindow() {
      return <div className={styles.container}>...</div>
  }
  ```
- **Avoid inline styles** - use CSS for maintainability
- **Responsive design**: Mobile-first media queries
  ```css
  .toolbar {
      display: flex;
      gap: 8px;
  }
  @media (max-width: 600px) {
      .toolbar {
          flex-direction: column;
      }
  }
  ```

### Error Handling
- **Try-catch in async functions**:
  ```typescript
  async function saveImage(blob: Blob) {
      try {
          await window.wails.Invoke('FileService.SaveImage', {
              path: '/tmp/image.png',
              data: await blob.arrayBuffer()
          })
      } catch (error) {
          const message = error instanceof Error
              ? error.message
              : 'Unknown error'
          showToast(`Save failed: ${message}`, 'error')
      }
  }
  ```
- **User-facing errors** - brief, actionable messages
- **Log errors** to console in dev mode:
  ```typescript
  if (import.meta.env.DEV) {
      console.error('Detailed error:', error)
  }
  ```

### Wails Integration
- **Wrap Wails calls** in services:
  ```typescript
  // services/capture.ts
  export const captureService = {
      fullscreen: () => window.wails.Invoke('CaptureService.FullScreen'),
      activeDisplay: () => window.wails.Invoke('CaptureService.ActiveDisplay')
  }

  // In component
  const image = await captureService.fullscreen()
  ```
- **Type safety**: Define return types
  ```typescript
  interface CaptureResult {
      data: string // base64
      width: number
      height: number
  }

  function useCapture() {
      const [result, setResult] = useState<CaptureResult | null>(null)
      // ...
  }
  ```

### Testing
- **Unit tests**: Jest with React Testing Library
- **Test naming**: `describe` blocks mirror component structure
  ```typescript
  describe('EditorWindow', () => {
      it('should undo annotation on Ctrl+Z', () => {
          // Test implementation
      })

      it('should save image when save button clicked', async () => {
          // Test implementation
      })
  })
  ```
- **Mock Wails**: Mock `window.wails` in tests
  ```typescript
  beforeEach(() => {
      window.wails = {
          Invoke: jest.fn()
      } as any
  })
  ```

---

## 3. Project-Wide Standards

### Commit Messages
- **Format**: Conventional Commits
  ```
  feat: add rectangle annotation tool
  fix: prevent undo on empty state
  docs: update README with install steps
  refactor: extract canvas logic to hooks
  test: add tests for hotkey registration
  chore: update dependencies
  ```
- **Keep concise** - first line ≤50 chars
- **Reference issues**: `fix: #123`
- **No AI references** - keep professional

### File Naming
- **Go**: snake_case (`capture_darwin.go`, `file_service.go`)
- **TypeScript/React**: kebab-case (`editor-window.tsx`, `use-hotkey.ts`)
- **Styles**: kebab-case matching component (`EditorWindow.module.css`)
- **Be descriptive**: `hotkey-registration-service.ts` better than `service.ts`

### Documentation
- **Inline comments**: For "why" not "what"
  ```go
  // Defer tray creation until after app launch to fix visibility on Big Sur
  dispatch_after(delay, queue, ^{ create_status_item() })
  ```
- **Function docstrings**: Follow language conventions
  ```go
  // CaptureFullscreen captures the entire display and returns PNG bytes.
  // Returns an error if screen recording permission is denied on macOS.
  func (s *Service) CaptureFullscreen(ctx context.Context) ([]byte, error)
  ```
- **README files**: At directory level if complex
  ```
  services/
  ├── README.md     # Service architecture overview
  ├── capture/
  └── file/
  ```

### Dependencies
- **Go**: Minimal external deps (currently Wails + golang.design/x/hotkey)
- **npm**: Use `npm install` only, no custom yarn/pnpm
- **Updates**: Test thoroughly before upgrading major versions
- **Security**: Run `npm audit` before releases

### Code Review Checklist
- ✅ Builds without errors (`go mod tidy`, `npm run build`)
- ✅ Tests pass (`go test ./...`, `npm test`)
- ✅ Lint passes (`golangci-lint`, ESLint)
- ✅ No hardcoded secrets (.env, API keys)
- ✅ Error handling complete (no silent failures)
- ✅ Platform-specific code properly isolated
- ✅ Comments explain "why" for complex logic
- ✅ Types checked (Go build, TypeScript strict mode)

---

## 4. Example: Adding a New Service

### Go Service Example
```go
// internal/service/clipboard/clipboard.go
package clipboard

import (
    "context"
    "fmt"
)

// Service handles clipboard operations
type Service interface {
    Copy(ctx context.Context, data []byte, format string) error
}

// New creates a platform-specific clipboard service
func New() Service {
    return newPlatformService()
}
```

```go
// internal/service/clipboard/clipboard_darwin.go
//go:build darwin

package clipboard

import (
    "context"
    "os/exec"
)

type darwinService struct{}

func newPlatformService() Service {
    return &darwinService{}
}

func (s *darwinService) Copy(ctx context.Context, data []byte, format string) error {
    // macOS implementation using pbcopy
    cmd := exec.CommandContext(ctx, "pbcopy")
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return fmt.Errorf("clipboard pipe failed: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("clipboard command failed: %w", err)
    }

    if _, err := stdin.Write(data); err != nil {
        return fmt.Errorf("clipboard write failed: %w", err)
    }
    stdin.Close()

    return cmd.Wait()
}
```

### TypeScript Hook Example
```typescript
// frontend/src/hooks/use-editor-undo.ts
import { useCallback } from 'react'
import { useEditorStore } from '../store/editor-store'

export function useEditorUndo() {
    const { undoStack, redo, undo: undoAction } = useEditorStore()

    const canUndo = undoStack.length > 0

    const handleUndo = useCallback(() => {
        if (!canUndo) return
        undoAction()
    }, [canUndo, undoAction])

    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'z') {
                e.preventDefault()
                handleUndo()
            }
        }

        window.addEventListener('keydown', handleKeyDown)
        return () => window.removeEventListener('keydown', handleKeyDown)
    }, [handleUndo])

    return { canUndo, undo: handleUndo }
}
```

---

## 5. Quick Reference

| Language | Naming | Max LOC | Build Tool | Lint |
|----------|--------|---------|-----------|------|
| Go | snake_case | 200 | `go build` | `golangci-lint` |
| TypeScript | kebab-case | 200 | `npm run build` | `ESLint` |
| React Components | PascalCase files | 200 | Vite | ESLint + Prettier |
| CSS Modules | kebab-case | N/A | Vite | Stylelint |

---

**Last Updated**: 2025-02-01
**Version**: 1.0
