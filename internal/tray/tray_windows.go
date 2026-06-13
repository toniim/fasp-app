//go:build windows

package tray

import (
	"os"

	"github.com/getlantern/systray"
)

type windowsManager struct {
	callback    Callback
	initialized bool
	iconData    []byte
	quitChan    chan struct{}
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
	mShow := systray.AddMenuItem("Show Window", "Show main window")
	mHide := systray.AddMenuItem("Hide Window", "Hide main window")
	systray.AddSeparator()
	mSettings := systray.AddMenuItem("Settings...", "Open settings")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Fasp")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mCapture.ClickedCh:
				if m.callback != nil {
					m.callback(ActionCapture)
				}
			case <-mOpenImage.ClickedCh:
				if m.callback != nil {
					m.callback(ActionOpenImage)
				}
			case <-mShow.ClickedCh:
				if m.callback != nil {
					m.callback(ActionShowWindow)
				}
			case <-mHide.ClickedCh:
				if m.callback != nil {
					m.callback(ActionHideWindow)
				}
			case <-mSettings.ClickedCh:
				if m.callback != nil {
					m.callback(ActionSettings)
				}
			case <-mQuit.ClickedCh:
				if m.callback != nil {
					m.callback(ActionQuit)
				}
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
