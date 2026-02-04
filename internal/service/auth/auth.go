package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/heytonyne/grabix/internal/model"
)

// authService implements the Service interface
type authService struct {
	config        *model.AuthConfig
	tokenManager  *TokenManager
	currentUser   *model.User
	codeVerifier  string
	callbackURL   string
	mu            sync.RWMutex
	server        *http.Server
	callbackDone  chan error
	callbackCode  string
	callbackError string
}

// New creates a new auth service instance
func New() Service {
	config := &model.AuthConfig{
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
		AuthorizeURL: os.Getenv("OAUTH_AUTHORIZE_URL"),
		TokenURL:     os.Getenv("OAUTH_TOKEN_URL"),
		UserInfoURL:  os.Getenv("OAUTH_USER_INFO_URL"),
	}

	tokenManager, err := NewTokenManager()
	if err != nil {
		println("Failed to create token manager:", err.Error())
	}

	return &authService{
		config:       config,
		tokenManager: tokenManager,
	}
}

// StartLogin initiates the OAuth login flow
func (s *authService) StartLogin() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cleanup any previous server and reset state
	if s.server != nil {
		s.server.Shutdown(context.Background())
		s.server = nil
	}
	s.callbackCode = ""
	s.callbackError = ""

	// Generate PKCE code verifier and challenge
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("failed to generate code verifier: %w", err)
	}
	s.codeVerifier = verifier
	challenge := GenerateCodeChallenge(verifier)

	// Start local callback server on fixed port for OAuth redirect
	const callbackPort = 19847
	s.callbackURL = fmt.Sprintf("http://localhost:%d/callback", callbackPort)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if err != nil {
		return "", fmt.Errorf("failed to start callback server on port %d: %w", callbackPort, err)
	}
	s.callbackDone = make(chan error, 1)

	// Setup HTTP server for callback
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleOAuthCallback)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			println("Callback server error:", err.Error())
		}
	}()

	// Build authorization URL
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&code_challenge=%s&code_challenge_method=S256",
		s.config.AuthorizeURL,
		url.QueryEscape(s.config.ClientID),
		url.QueryEscape(s.callbackURL),
		url.QueryEscape("email offline_access profile"),
		url.QueryEscape(challenge),
	)

	return authURL, nil
}

// handleOAuthCallback processes the OAuth callback request
func (s *authService) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	errMsg := r.URL.Query().Get("error")

	if errMsg != "" {
		s.callbackError = errMsg
		s.callbackDone <- fmt.Errorf("oauth error: %s", errMsg)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", errMsg)
		return
	}

	if code == "" {
		s.callbackError = "no authorization code received"
		s.callbackDone <- fmt.Errorf("no authorization code received")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>No authorization code received</p><p>You can close this window.</p></body></html>")
		return
	}

	s.callbackCode = code
	s.callbackDone <- nil

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "<html><body><h1>Authentication Successful!</h1><p>You can close this window and return to the app.</p></body></html>")

	// Shutdown server after successful callback
	go func() {
		time.Sleep(1 * time.Second)
		s.server.Shutdown(context.Background())
	}()
}

// HandleCallback processes the OAuth callback with the authorization code
func (s *authService) HandleCallback(code string) (*model.User, error) {
	s.mu.Lock()
	verifier := s.codeVerifier
	callbackURL := s.callbackURL
	s.mu.Unlock()

	// Wait for callback to complete (with timeout)
	select {
	case err := <-s.callbackDone:
		if err != nil {
			return nil, err
		}
		code = s.callbackCode
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("callback timeout")
	}

	// Exchange authorization code for tokens
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", callbackURL)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequest("POST", s.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s - %s", resp.Status, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Fetch user info
	user, err := s.fetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	// Save tokens
	token := &model.AuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		UserID:       user.ID,
	}

	if err := s.tokenManager.Save(token); err != nil {
		return nil, fmt.Errorf("failed to save tokens: %w", err)
	}

	s.mu.Lock()
	s.currentUser = user
	s.mu.Unlock()

	return user, nil
}

// fetchUserInfo retrieves user information from the OAuth provider
func (s *authService) fetchUserInfo(accessToken string) (*model.User, error) {
	req, err := http.NewRequest("GET", s.config.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read user info response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info request failed: %s - %s", resp.Status, string(body))
	}

	var user model.User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &user, nil
}

// RefreshToken refreshes the access token using the refresh token
func (s *authService) RefreshToken() error {
	token, err := s.tokenManager.GetTokens()
	if err != nil {
		return fmt.Errorf("failed to get tokens: %w", err)
	}

	if token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", token.RefreshToken)
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)

	req, err := http.NewRequest("POST", s.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Update tokens
	newToken := &model.AuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		UserID:       token.UserID,
	}

	// Keep old refresh token if new one not provided
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = token.RefreshToken
	}

	return s.tokenManager.Save(newToken)
}

// GetCurrentUser returns the currently authenticated user
func (s *authService) GetCurrentUser() (*model.User, error) {
	s.mu.RLock()
	if s.currentUser != nil {
		user := s.currentUser
		s.mu.RUnlock()
		return user, nil
	}
	s.mu.RUnlock()

	// Try to load from token and fetch user info
	token, err := s.tokenManager.GetTokens()
	if err != nil {
		return nil, fmt.Errorf("not authenticated")
	}

	// Check if token needs refresh
	if s.tokenManager.NeedsRefresh(token) {
		if err := s.RefreshToken(); err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		// Reload token after refresh
		token, err = s.tokenManager.GetTokens()
		if err != nil {
			return nil, err
		}
	}

	// Fetch fresh user info
	user, err := s.fetchUserInfo(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	s.mu.Lock()
	s.currentUser = user
	s.mu.Unlock()

	return user, nil
}

// Logout removes authentication tokens
func (s *authService) Logout() error {
	s.mu.Lock()
	s.currentUser = nil
	s.mu.Unlock()

	return s.tokenManager.Clear()
}

// IsLoggedIn checks if user is currently authenticated
func (s *authService) IsLoggedIn() bool {
	token, err := s.tokenManager.GetTokens()
	if err != nil {
		return false
	}

	return !s.tokenManager.IsExpired(token)
}

// GetAccessToken returns the current access token (for API calls)
func (s *authService) GetAccessToken() (string, error) {
	token, err := s.tokenManager.GetTokens()
	if err != nil {
		return "", fmt.Errorf("not authenticated")
	}

	// Check if token needs refresh
	if s.tokenManager.NeedsRefresh(token) {
		if err := s.RefreshToken(); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		// Reload token after refresh
		token, err = s.tokenManager.GetTokens()
		if err != nil {
			return "", err
		}
	}

	return token.AccessToken, nil
}
