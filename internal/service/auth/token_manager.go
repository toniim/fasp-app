package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/heytonyne/grabix/internal/model"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// Token file location
	tokenFileName = "tokens.enc"
	// PBKDF2 iterations
	pbkdf2Iterations = 100000
	// Key length for AES-256
	keyLength = 32
	// Salt length
	saltLength = 16
)

// TokenManager handles secure token storage and retrieval
type TokenManager struct {
	tokenPath string
	mu        sync.RWMutex
}

// NewTokenManager creates a new token manager
func NewTokenManager() (*TokenManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".grabix")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	return &TokenManager{
		tokenPath: filepath.Join(configDir, tokenFileName),
	}, nil
}

// Save encrypts and saves authentication tokens
func (tm *TokenManager) Save(token *model.AuthToken) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Serialize token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Encrypt data
	encrypted, err := tm.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Write to file with restricted permissions
	if err := os.WriteFile(tm.tokenPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// Load decrypts and loads authentication tokens
func (tm *TokenManager) Load() (*model.AuthToken, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Check if file exists
	if _, err := os.Stat(tm.tokenPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("token file not found")
	}

	// Read encrypted file
	encrypted, err := os.ReadFile(tm.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	// Decrypt data
	data, err := tm.decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	// Deserialize token
	var token model.AuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// Clear removes stored tokens
func (tm *TokenManager) Clear() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err := os.Remove(tm.tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove token file: %w", err)
	}

	return nil
}

// GetTokens returns the current tokens
func (tm *TokenManager) GetTokens() (*model.AuthToken, error) {
	return tm.Load()
}

// NeedsRefresh checks if the token needs to be refreshed
// Returns true if token expires within 5 minutes
func (tm *TokenManager) NeedsRefresh(token *model.AuthToken) bool {
	if token == nil {
		return true
	}
	return time.Until(token.ExpiresAt) < 5*time.Minute
}

// IsExpired checks if the token is expired
func (tm *TokenManager) IsExpired(token *model.AuthToken) bool {
	if token == nil {
		return true
	}
	return time.Now().After(token.ExpiresAt)
}

// encrypt encrypts data using AES-256-GCM with PBKDF2 key derivation
func (tm *TokenManager) encrypt(plaintext []byte) ([]byte, error) {
	// Generate machine-specific key
	key, err := tm.deriveKey()
	if err != nil {
		return nil, err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-256-GCM with PBKDF2 key derivation
func (tm *TokenManager) decrypt(ciphertext []byte) ([]byte, error) {
	// Generate machine-specific key
	key, err := tm.deriveKey()
	if err != nil {
		return nil, err
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveKey derives an encryption key from machine-specific data using PBKDF2
func (tm *TokenManager) deriveKey() ([]byte, error) {
	// Get machine-specific identifier
	machineID, err := tm.getMachineID()
	if err != nil {
		return nil, err
	}

	// Use a fixed salt derived from machine ID for consistent key generation
	// In production, you might want to store the salt separately
	salt := sha256.Sum256([]byte(machineID))

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(machineID), salt[:], pbkdf2Iterations, keyLength, sha256.New)
	return key, nil
}

// getMachineID generates a machine-specific identifier
func (tm *TokenManager) getMachineID() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	// Combine hostname and username for machine ID
	machineID := fmt.Sprintf("%s:%s", hostname, username)
	return machineID, nil
}
