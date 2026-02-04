package auth

import "github.com/heytonyne/grabix/internal/model"

// Service defines the interface for authentication operations
type Service interface {
	// StartLogin initiates the OAuth login flow and returns the authorization URL
	StartLogin() (string, error)

	// HandleCallback processes the OAuth callback with the authorization code
	HandleCallback(code string) (*model.User, error)

	// RefreshToken refreshes the access token using the refresh token
	RefreshToken() error

	// GetCurrentUser returns the currently authenticated user
	GetCurrentUser() (*model.User, error)

	// Logout removes authentication tokens
	Logout() error

	// IsLoggedIn checks if user is currently authenticated
	IsLoggedIn() bool

	// GetAccessToken returns the current access token (for API calls)
	GetAccessToken() (string, error)
}
