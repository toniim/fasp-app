//go:build windows

package tray

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/getlantern/systray"
)

// maxRecentItems is how many recents the submenu can show (Xếp: 10 last saves).
const maxRecentItems = 10

type windowsManager struct {
	callback    Callback
	initialized bool
	iconData    []byte
	quitChan    chan struct{}

	mu          sync.Mutex
	recents     []string
	recentItems []*systray.MenuItem
	recentShown []bool
	menuReady   bool
}

func NewManager() Manager {
	return &windowsManager{
		quitChan: make(chan struct{}),
	}
}

func (m *windowsManager) Setup(callback Callback) error {
	m.callback = callback
	m.initialized = true

	// Start systray in a goroutine
	go systray.Run(m.onReady, m.onExit)

	return nil
}

// SetRecents stores the list and refreshes the submenu when it exists yet.
func (m *windowsManager) SetRecents(paths []string) error {
	m.mu.Lock()
	m.recents = append([]string(nil), paths...)
	m.mu.Unlock()

	m.applyRecents()
	return nil
}

func (m *windowsManager) onReady() {
	// Set default icon if we have icon data
	if len(m.iconData) > 0 {
		systray.SetIcon(m.iconData)
	}

	systray.SetTitle("Fasp")
	systray.SetTooltip("Fasp - Screenshot Tool")

	// Create menu items
	mCapture := systray.AddMenuItem("Capture Screenshot", "Take a screenshot")
	mOpenImage := systray.AddMenuItem("Open Image...", "Open an image for annotation")
	systray.AddSeparator()

	// Recents submenu: Open Folder, divider, then the last saved screenshots.
	mRecents := systray.AddMenuItem("Recents", "Recently saved screenshots")
	mOpenFolder := mRecents.AddSubMenuItem("Open Folder", "Open the folder with your screenshots")
	// systray has no separator API for submenus, so use a disabled rule line.
	mDivider := mRecents.AddSubMenuItem(strings.Repeat("─", 18), "")
	mDivider.Disable()

	for i := 0; i < maxRecentItems; i++ {
		item := mRecents.AddSubMenuItem("", "")
		item.Hide()
		m.recentItems = append(m.recentItems, item)
		m.recentShown = append(m.recentShown, false)
	}

	systray.AddSeparator()
	mShow := systray.AddMenuItem("Show Window", "Show main window")
	mHide := systray.AddMenuItem("Hide Window", "Hide main window")
	systray.AddSeparator()
	mSettings := systray.AddMenuItem("Settings...", "Open settings")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Fasp")

	m.mu.Lock()
	m.menuReady = true
	m.mu.Unlock()
	m.applyRecents()

	// Each recent slot is clicked through the same channel; resolve the path at
	// click time so refreshes (new saves) don't need new menu items.
	for idx, item := range m.recentItems {
		go func(i int, it *systray.MenuItem) {
			for {
				select {
				case <-it.ClickedCh:
					path := m.recentAt(i)
					if path != "" && m.callback != nil {
						m.callback(ActionOpenRecent, path)
					}
				case <-m.quitChan:
					return
				}
			}
		}(idx, item)
	}

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mCapture.ClickedCh:
				m.emit(ActionCapture, "")
			case <-mOpenImage.ClickedCh:
				m.emit(ActionOpenImage, "")
			case <-mOpenFolder.ClickedCh:
				m.emit(ActionOpenRecentsFolder, m.recentFolder())
			case <-mShow.ClickedCh:
				m.emit(ActionShowWindow, "")
			case <-mHide.ClickedCh:
				m.emit(ActionHideWindow, "")
			case <-mSettings.ClickedCh:
				m.emit(ActionSettings, "")
			case <-mQuit.ClickedCh:
				m.emit(ActionQuit, "")
				systray.Quit()
				return
			case <-m.quitChan:
				systray.Quit()
				return
			}
		}
	}()
}

func (m *windowsManager) onExit() {
	// Cleanup
}

// applyRecents syncs the submenu items with the stored list.
func (m *windowsManager) applyRecents() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.menuReady {
		return
	}

	paths := m.recents
	if len(paths) > len(m.recentItems) {
		paths = paths[:len(m.recentItems)]
	}

	println("[DEBUG] tray recents refresh:", len(paths), "item(s)")

	for i, item := range m.recentItems {
		if i >= len(paths) {
			// Only hide once: repeated Hide() on an already-removed item logs errors.
			if m.recentShown[i] {
				item.Hide()
				m.recentShown[i] = false
			}
			continue
		}
		item.SetTitle(menuLabel(paths[i]))
		item.SetTooltip(paths[i])
		item.Show()
		m.recentShown[i] = true
	}
}

func (m *windowsManager) recentAt(index int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.recents) {
		return ""
	}
	return m.recents[index]
}

func (m *windowsManager) recentFolder() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.recents {
		if _, err := os.Stat(p); err == nil {
			return filepath.Dir(p)
		}
	}
	return ""
}

func (m *windowsManager) emit(action Action, arg string) {
	if m.callback != nil {
		m.callback(action, arg)
	}
}

// menuLabel turns a full path into the short label shown in the menu.
func menuLabel(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func (m *windowsManager) UpdateIcon(iconData []byte) error {
	m.iconData = iconData
	if m.initialized && len(iconData) > 0 {
		systray.SetIcon(iconData)
	}
	return nil
}

func (m *windowsManager) SetIconFromFile(iconPath string) error {
	if iconPath == "" {
		return nil
	}

	data, err := os.ReadFile(iconPath)
	if err != nil {
		return err
	}

	return m.UpdateIcon(data)
}

func (m *windowsManager) Quit() {
	if !m.initialized {
		return
	}

	select {
	case m.quitChan <- struct{}{}:
	default:
	}

	m.initialized = false
}
