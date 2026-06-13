package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/heytonyne/fasp/internal/model"
	"github.com/heytonyne/fasp/internal/service/settings"
)

// uploadService implements the Service interface.
//
// It talks to a fasp server using the 3-step upload flow
// (init -> presigned R2 PUT -> complete) and authorizes every request with a
// fasp API key (fsk_live_*) via the `Authorization: Bearer` header. The server
// URL and API key are read live from the settings service on each call so the
// user can change them at runtime without restarting the app.
type uploadService struct {
	settings settings.Service
}

// New creates a new upload service instance backed by the settings service.
func New(settingsSvc settings.Service) Service {
	return &uploadService{settings: settingsSvc}
}

// credentials returns the configured server base URL and API key.
func (s *uploadService) credentials() (string, string, error) {
	cfg, err := s.settings.GetAll()
	if err != nil {
		return "", "", fmt.Errorf("failed to read settings: %w", err)
	}

	serverURL := strings.TrimRight(strings.TrimSpace(cfg.ServerURL), "/")
	apiKey := strings.TrimSpace(cfg.APIKey)

	if serverURL == "" {
		return "", "", fmt.Errorf("server URL not configured")
	}
	if apiKey == "" {
		return "", "", fmt.Errorf("API key not configured")
	}
	return serverURL, apiKey, nil
}

// Upload uploads image data using the full 3-step flow.
func (s *uploadService) Upload(data []byte, filename string) (*model.UploadResult, error) {
	initResp, err := s.Init(filename, int64(len(data)), "image/png")
	if err != nil {
		return nil, err
	}

	// PUT raw bytes directly to the presigned R2 URL (no auth header needed).
	req, err := http.NewRequest("PUT", initResp.UploadURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "image/png")
	req.ContentLength = int64(len(data))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	completeResp, err := s.Complete(initResp.FileID)
	if err != nil {
		return nil, err
	}

	return &model.UploadResult{
		URL:       completeResp.PublicURL,
		ID:        initResp.FileID,
		Timestamp: time.Now(),
	}, nil
}

// Init initiates a file upload and returns a presigned upload URL.
func (s *uploadService) Init(filename string, size int64, contentType string) (*InitResponse, error) {
	serverURL, apiKey, err := s.credentials()
	if err != nil {
		return nil, err
	}

	initReq := InitRequest{
		Filename: filename,
		Size:     size,
		MimeType: contentType,
		IsPublic: true,
	}

	jsonData, err := json.Marshal(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal init request: %w", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/api/upload/init", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create init request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send init request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read init response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("init request failed: %s - %s", resp.Status, string(body))
	}

	var initResp InitResponse
	if err := json.Unmarshal(body, &initResp); err != nil {
		return nil, fmt.Errorf(
			"init response was not JSON (status %s, content-type %q, final-url %s): %s",
			resp.Status, resp.Header.Get("Content-Type"), resp.Request.URL.String(), bodySnippet(body),
		)
	}

	return &initResp, nil
}

// bodySnippet returns a single-line, truncated view of a response body for
// diagnostics (avoids dumping a full HTML page into the error).
func bodySnippet(body []byte) string {
	const max = 300
	s := strings.TrimSpace(string(body))
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		return "(empty body)"
	}
	return s
}

// Complete completes a file upload and returns public URLs.
func (s *uploadService) Complete(fileID string) (*CompleteResponse, error) {
	serverURL, apiKey, err := s.credentials()
	if err != nil {
		return nil, err
	}

	completeReq := CompleteRequest{FileID: fileID}

	jsonData, err := json.Marshal(completeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal complete request: %w", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/api/upload/complete", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create complete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send complete request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read complete response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("complete request failed: %s - %s", resp.Status, string(body))
	}

	var completeResp CompleteResponse
	if err := json.Unmarshal(body, &completeResp); err != nil {
		return nil, fmt.Errorf(
			"complete response was not JSON (status %s, content-type %q, final-url %s): %s",
			resp.Status, resp.Header.Get("Content-Type"), resp.Request.URL.String(), bodySnippet(body),
		)
	}

	return &completeResp, nil
}

// IsConfigured reports whether a server URL and API key are set.
func (s *uploadService) IsConfigured() bool {
	_, _, err := s.credentials()
	return err == nil
}

// TestConnection verifies the configured server URL + API key are valid by
// hitting an authenticated read endpoint (files:read scope).
func (s *uploadService) TestConnection() error {
	serverURL, apiKey, err := s.credentials()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", serverURL+"/api/files?limit=1", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach server: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("invalid API key")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response: %s - %s", resp.Status, string(body))
	}
}
