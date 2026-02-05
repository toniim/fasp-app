package hotkey

import (
	"fmt"
	"strings"
	"sync"

	"golang.design/x/hotkey"
)

// serviceImpl implements the Service interface
type serviceImpl struct {
	mu       sync.RWMutex
	hotkeys  map[string]*hotkeyInstance
	running  bool
	stopChan chan struct{}
}

type hotkeyInstance struct {
	hk       *hotkey.Hotkey
	callback func()
	stopChan chan struct{}
}

// New creates a new hotkey service instance
func New() Service {
	return &serviceImpl{
		hotkeys:  make(map[string]*hotkeyInstance),
		stopChan: make(chan struct{}),
	}
}

// Register registers a hotkey with a callback
func (s *serviceImpl) Register(key string, callback func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse hotkey string (e.g., "Ctrl+F11")
	mods, k, err := parseHotkey(key)
	if err != nil {
		return fmt.Errorf("failed to parse hotkey %s: %w", key, err)
	}

	fmt.Printf("Registering hotkey: %s -> key=%v, mods=%v\n", key, k, mods)

	// Create hotkey instance
	hk := hotkey.New(mods, k)

	instance := &hotkeyInstance{
		hk:       hk,
		callback: callback,
		stopChan: make(chan struct{}),
	}

	s.hotkeys[key] = instance

	// Register immediately if service is running
	if s.running {
		if err := hk.Register(); err != nil {
			return fmt.Errorf("failed to register hotkey: %w", err)
		}

		// Start listening
		go s.listenHotkey(instance)
	}

	return nil
}

// Unregister unregisters a hotkey
func (s *serviceImpl) Unregister(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance, exists := s.hotkeys[key]
	if !exists {
		return fmt.Errorf("hotkey %s not registered", key)
	}

	// Stop listening - close channel only if not already closed
	select {
	case <-instance.stopChan:
		// Already closed
	default:
		close(instance.stopChan)
	}

	// Unregister the hotkey
	if err := instance.hk.Unregister(); err != nil {
		return fmt.Errorf("failed to unregister hotkey: %w", err)
	}

	delete(s.hotkeys, key)
	fmt.Printf("Unregistered hotkey: %s\n", key)
	return nil
}

// UpdateHotkey updates a hotkey binding
func (s *serviceImpl) UpdateHotkey(oldKey, newKey string, callback func()) error {
	if err := s.Unregister(oldKey); err != nil {
		// Ignore error if hotkey doesn't exist
		if !strings.Contains(err.Error(), "not registered") {
			return err
		}
	}
	return s.Register(newKey, callback)
}

// Start starts listening for hotkeys
func (s *serviceImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("hotkey service already running")
	}

	s.running = true
	fmt.Println("Starting hotkey service...")

	// Register all hotkeys
	for key, instance := range s.hotkeys {
		// Recreate stopChan if it was closed
		select {
		case <-instance.stopChan:
			instance.stopChan = make(chan struct{})
		default:
		}

		if err := instance.hk.Register(); err != nil {
			fmt.Printf("Failed to register hotkey %s: %v\n", key, err)
			continue
		}

		// Start listening
		go s.listenHotkey(instance)
	}

	return nil
}

// Stop stops listening for hotkeys
func (s *serviceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	fmt.Println("Stopping hotkey service...")
	s.running = false

	// Unregister all hotkeys
	for _, instance := range s.hotkeys {
		// Close stopChan only if not already closed
		select {
		case <-instance.stopChan:
			// Already closed
		default:
			close(instance.stopChan)
		}
		instance.hk.Unregister()
	}

	return nil
}

// listenHotkey listens for a specific hotkey
func (s *serviceImpl) listenHotkey(instance *hotkeyInstance) {
	for {
		select {
		case <-instance.hk.Keydown():
			fmt.Printf("✅ Hotkey triggered!\n")
			if instance.callback != nil {
				instance.callback()
			}
		case <-instance.stopChan:
			return
		}
	}
}

// parseHotkey parses a hotkey string like "Ctrl+F11" into modifiers and key
func parseHotkey(hotkeyStr string) ([]hotkey.Modifier, hotkey.Key, error) {
	parts := strings.Split(strings.ToLower(hotkeyStr), "+")
	if len(parts) == 0 {
		return nil, 0, fmt.Errorf("invalid hotkey format")
	}

	var mods []hotkey.Modifier
	var key hotkey.Key
	var keyFound bool

	for _, part := range parts {
		part = strings.TrimSpace(part)

		switch part {
		case "ctrl", "control":
			mods = append(mods, hotkey.ModCtrl)
		case "shift":
			mods = append(mods, hotkey.ModShift)
		case "alt", "option":
			mods = append(mods, modAlt)
		case "cmd", "command", "meta", "win", "super":
			mods = append(mods, modMeta)
		default:
			// This is the main key
			k, err := parseKey(part)
			if err != nil {
				return nil, 0, err
			}
			key = k
			keyFound = true
		}
	}

	if !keyFound {
		return nil, 0, fmt.Errorf("no main key found in hotkey")
	}

	return mods, key, nil
}

// parseKey converts a key string to hotkey.Key
func parseKey(keyStr string) (hotkey.Key, error) {
	keyStr = strings.ToLower(keyStr)

	// Function keys F1-F12
	switch keyStr {
	case "f1":
		return hotkey.KeyF1, nil
	case "f2":
		return hotkey.KeyF2, nil
	case "f3":
		return hotkey.KeyF3, nil
	case "f4":
		return hotkey.KeyF4, nil
	case "f5":
		return hotkey.KeyF5, nil
	case "f6":
		return hotkey.KeyF6, nil
	case "f7":
		return hotkey.KeyF7, nil
	case "f8":
		return hotkey.KeyF8, nil
	case "f9":
		return hotkey.KeyF9, nil
	case "f10":
		return hotkey.KeyF10, nil
	case "f11":
		return hotkey.KeyF11, nil
	case "f12":
		return hotkey.KeyF12, nil
	case "f13":
		return hotkey.KeyF13, nil
	case "f14":
		return hotkey.KeyF14, nil
	case "f15":
		return hotkey.KeyF15, nil
	case "f16":
		return hotkey.KeyF16, nil
	case "f17":
		return hotkey.KeyF17, nil
	case "f18":
		return hotkey.KeyF18, nil
	case "f19":
		return hotkey.KeyF19, nil
	case "f20":
		return hotkey.KeyF20, nil
	}

	// Letters
	if len(keyStr) == 1 && keyStr[0] >= 'a' && keyStr[0] <= 'z' {
		return hotkey.Key(int32(keyStr[0]-'a') + int32(hotkey.KeyA)), nil
	}

	// Numbers
	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		return hotkey.Key(int32(keyStr[0]-'0') + int32(hotkey.Key0)), nil
	}

	// Special keys
	switch keyStr {
	case "space":
		return hotkey.KeySpace, nil
	case "enter", "return":
		return hotkey.KeyReturn, nil
	case "tab":
		return hotkey.KeyTab, nil
	case "backspace", "delete":
		return hotkey.KeyDelete, nil
	case "esc", "escape":
		return hotkey.KeyEscape, nil
	case "left":
		return hotkey.KeyLeft, nil
	case "right":
		return hotkey.KeyRight, nil
	case "up":
		return hotkey.KeyUp, nil
	case "down":
		return hotkey.KeyDown, nil
	// Additional special keys
	case "printscreen", "prtsc", "prtscn", "print":
		return hotkey.Key(0x2C), nil // VK_SNAPSHOT on Windows
	case "help":
		return hotkey.Key(0x72), nil
	case "home":
		return hotkey.Key(0x73), nil
	case "pageup":
		return hotkey.Key(0x74), nil
	case "forwarddelete", "del":
		return hotkey.Key(0x75), nil
	case "end":
		return hotkey.Key(0x77), nil
	case "pagedown":
		return hotkey.Key(0x79), nil
	case "insert":
		return hotkey.Key(0x2D), nil // VK_INSERT
	case "pause":
		return hotkey.Key(0x13), nil // VK_PAUSE
	case "scrolllock":
		return hotkey.Key(0x91), nil // VK_SCROLL
	case "numlock":
		return hotkey.Key(0x90), nil // VK_NUMLOCK
	}

	return 0, fmt.Errorf("unknown key: %s", keyStr)
}
