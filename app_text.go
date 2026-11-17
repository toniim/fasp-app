package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/heytonyne/fasp/internal/service/capture"
	"github.com/heytonyne/fasp/internal/service/ocr"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// TextResult is the outcome of recognising the text in an image.
type TextResult struct {
	Text  string `json:"text"`
	Lines int    `json:"lines"`
}

// ToastEvent is the payload of the "toast" event: a short message the editor
// shows, for work that started outside it (a hotkey).
type ToastEvent struct {
	Message string `json:"message"`
	Type    string `json:"type"` // success | error | info
}

// textSummary is the one-line result shown after text was copied.
func textSummary(lines int) string {
	if lines == 1 {
		return "Copied 1 line of text"
	}
	return fmt.Sprintf("Copied %d lines of text", lines)
}

const noTextMessage = "No text found"

// ocrError turns an engine error into a message the user can act on.
func ocrError(err error) error {
	switch {
	case errors.Is(err, ocr.ErrNoLanguage):
		if runtime.GOOS == "windows" {
			return errors.New("No OCR language is installed. Add one in Windows Settings > Time & language > Language & region, then try again.")
		}
		return errors.New("No OCR language is available on this Mac.")
	case errors.Is(err, ocr.ErrUnsupported):
		return errors.New("Text recognition is not supported on this system.")
	default:
		return fmt.Errorf("text recognition failed: %w", err)
	}
}

// ocrLanguages is the language hint from the settings: nil means automatic.
func (a *App) ocrLanguages() []string {
	s, err := a.settingsService.GetAll()
	if err != nil || strings.TrimSpace(s.OCRLanguage) == "" {
		return nil
	}
	return []string{s.OCRLanguage}
}

// recogniseAndCopy reads the text of a PNG and puts it on the clipboard. When
// the image holds no text the clipboard is left alone and the result is empty.
func (a *App) recogniseAndCopy(png []byte) (*TextResult, error) {
	res, err := ocr.Recognize(png, a.ocrLanguages())
	if err != nil {
		return nil, ocrError(err)
	}
	if len(res.Lines) == 0 {
		return &TextResult{}, nil
	}
	if err := a.setClipboardText(res.Text); err != nil {
		return nil, fmt.Errorf("failed to copy the text: %w", err)
	}
	return &TextResult{Text: res.Text, Lines: len(res.Lines)}, nil
}

func (a *App) setClipboardText(text string) error {
	if a.copyTextFn != nil {
		return a.copyTextFn(text)
	}
	if !a.app.Clipboard.SetText(text) {
		return errors.New("the clipboard did not accept the text")
	}
	return nil
}

// CopyText recognises the text in an exported shot (PNG bytes) and copies it to
// the clipboard. Lines is 0 when no text was found.
func (a *App) CopyText(data []byte) (*TextResult, error) {
	return a.recogniseAndCopy(data)
}

// GetOCRLanguages lists the language tags the system OCR engine can read, for
// the language hint in Settings. The list is empty where the engine reports none.
func (a *App) GetOCRLanguages() ([]string, error) {
	langs, err := ocr.SupportedLanguages()
	if errors.Is(err, ocr.ErrUnsupported) {
		return nil, nil
	}
	return langs, err
}

// CaptureText lets the user drag a zone (a click takes the window under the
// cursor), reads its text and copies it to the clipboard. The editor does not
// open. It reports the result with a toast when the editor is showing, else with
// a system notification.
func (a *App) CaptureText() {
	if a.IsRecording() || !a.textCapturing.CompareAndSwap(false, true) {
		return
	}
	defer a.textCapturing.Store(false)

	msg, kind := a.captureText()
	if msg != "" {
		a.notifyUser(msg, kind)
	}
}

// captureText runs the capture and returns the message to show; an empty
// message means the user cancelled.
func (a *App) captureText() (msg, kind string) {
	if err := a.requireScreenRecording(); err != nil {
		return err.Error(), "error"
	}
	// The zone memory is for Repeat Last Zone; reading text does not change it.
	r, _, err := a.captureService.CaptureZone()
	if errors.Is(err, capture.ErrCancelled) {
		return "", ""
	}
	if err != nil {
		return fmt.Sprintf("Capture failed: %v", err), "error"
	}
	png, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return fmt.Sprintf("Capture failed: %v", err), "error"
	}
	res, err := a.recogniseAndCopy(png)
	if err != nil {
		return err.Error(), "error"
	}
	if res.Lines == 0 {
		return noTextMessage, "info"
	}
	return textSummary(res.Lines), "success"
}

// notifyUser shows a short message without opening anything: a toast inside the
// editor when it is on screen, otherwise a system notification.
func (a *App) notifyUser(message, kind string) {
	if a.window != nil && a.window.IsVisible() && !a.window.IsMinimised() {
		a.emit("toast", ToastEvent{Message: message, Type: kind})
		return
	}
	if a.notifier == nil {
		println("[notify]", message)
		return
	}
	ok, err := a.notifier.CheckNotificationAuthorization()
	if err == nil && !ok {
		ok, err = a.notifier.RequestNotificationAuthorization()
	}
	if err != nil || !ok {
		println("[notify] notifications are not allowed:", message)
		return
	}
	id, err := newRandomID()
	if err != nil {
		println("[error] notification id:", err.Error())
		return
	}
	err = a.notifier.SendNotification(notifications.NotificationOptions{
		ID:    id,
		Title: "Fasp",
		Body:  message,
	})
	if err != nil {
		println("[error] notification:", err.Error())
	}
}
