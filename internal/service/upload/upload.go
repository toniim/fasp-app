package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/heytonyne/grabix/internal/model"
	"github.com/heytonyne/grabix/internal/service/auth"
)

// uploadService implements the Service interface
type uploadService struct {
	apiHost     string
	authService auth.Service
}

// New creates a new upload service instance
func New() Service {
	return &uploadService{
		apiHost:     os.Getenv("API_HOST"),
		authService: auth.New(),
	}
}

// Upload uploads image data to the configured provider (legacy method)
func (s *uploadService) Upload(data []byte, filename string) (*model.UploadResult, error) {
	// Use new 3-step flow
	initResp, err := s.Init(filename, int64(len(data)), "image/png")
	if err != nil {
		return nil, err
	}

	// PUT file directly
	req, err := http.NewRequest("PUT", initResp.UploadURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "image/png")

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

	// Complete upload
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

// Init initiates a file upload and returns upload URL
func (s *uploadService) Init(filename string, size int64, contentType string) (*InitResponse, error) {
	if s.apiHost == "" {
		return nil, fmt.Errorf("API_HOST not configured")
	}

	// Get access token
	accessToken, err := s.authService.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	// Create init request
	initReq := InitRequest{
		Filename:    filename,
		Size:        size,
		MimeType: contentType,
	}

	jsonData, err := json.Marshal(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal init request: %w", err)
	}

	// Send POST request to init endpoint
	req, err := http.NewRequest("POST", s.apiHost+"/api/upload/init", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create init request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
		return nil, fmt.Errorf("failed to parse init response: %w", err)
	}

	return &initResp, nil
}

// Complete completes a file upload and returns public URLs
func (s *uploadService) Complete(fileID string) (*CompleteResponse, error) {
	if s.apiHost == "" {
		return nil, fmt.Errorf("API_HOST not configured")
	}

	// Get access token
	accessToken, err := s.authService.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("not authenticated: %w", err)
	}

	// Create complete request
	completeReq := CompleteRequest{
		FileID: fileID,
	}

	jsonData, err := json.Marshal(completeReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal complete request: %w", err)
	}

	// Send POST request to complete endpoint
	req, err := http.NewRequest("POST", s.apiHost+"/api/upload/complete", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create complete request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
		return nil, fmt.Errorf("failed to parse complete response: %w", err)
	}

	return &completeResp, nil
}

// IsConfigured checks if upload service is properly configured
func (s *uploadService) IsConfigured() bool {
	return s.apiHost != "" && s.authService.IsLoggedIn()
}

// GetProviders returns a list of available upload providers (legacy method)
func (s *uploadService) GetProviders() []string {
	return []string{"api"}
}

// SetProvider sets the active upload provider (legacy method)
func (s *uploadService) SetProvider(name string) error {
	return nil
}

// GetActiveProvider returns the currently active provider (legacy method)
func (s *uploadService) GetActiveProvider() string {
	return "api"
}
