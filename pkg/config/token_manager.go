package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	graphqlEndpoint = "https://graphql-mainnet.boop.works/graphql"
	privyEndpoint   = "https://auth.privy.io/api/v1/sessions"
)

// PrivyConfig holds Privy authentication tokens
type PrivyConfig struct {
	Authentication string // privy-authentication header value
	Token          string // privy-token header value
	RefreshToken   string // refresh token for Privy
	PrivyAppID     string // privy-app-id header value
	PrivyClientID  string // privy-ca-id header value
	PrivyClient    string // privy-client header value
}

// TokenResponse represents the response from the GraphQL login mutation
type TokenResponse struct {
	Data struct {
		LoginWithPrivy struct {
			Token string `json:"token"`
		} `json:"loginWithPrivy"`
	} `json:"data"`
}

// PrivyTokenResponse represents the response from the Privy token refresh
type PrivyTokenResponse struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Token            string `json:"token"`
	PrivyAccessToken string `json:"privy_access_token"`
	RefreshToken     string `json:"refresh_token"`
	IdentityToken    string `json:"identity_token"`
}

// TokenManager handles refreshing authentication tokens
type TokenManager struct {
	privyConfig  PrivyConfig
	graphqlToken string
	logger       *log.Logger
}

// NewTokenManager creates a new token manager
func NewTokenManager(privyAuth string, privyToken string, privyRefreshToken string, logger *log.Logger) *TokenManager {
	return &TokenManager{
		privyConfig: PrivyConfig{
			Authentication: privyAuth,
			Token:          privyToken,
			RefreshToken:   privyRefreshToken,
			PrivyAppID:     DefaultPrivyConfig.AppID,
			PrivyClientID:  DefaultPrivyConfig.ClientID,
			PrivyClient:    DefaultPrivyConfig.Client,
		},
		logger: logger,
	}
}

// NewTokenManagerWithPrivateKey creates a new token manager using a wallet private key
func NewTokenManagerWithPrivateKey(privateKey string, logger *log.Logger) (*TokenManager, error) {
	if logger == nil {
		logger = log.New(log.Writer(), "[TOKEN_MANAGER] ", log.LstdFlags)
	}

	// Get Privy tokens using the private key
	privyAuth, privyToken, privyRefreshToken, err := GetPrivyTokensWithPrivateKey(privateKey, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with private key: %w", err)
	}

	// Create token manager with obtained tokens
	tm := NewTokenManager(privyAuth, privyToken, privyRefreshToken, logger)

	// Immediately try to get GraphQL token
	err = tm.refreshGraphQLToken()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain GraphQL token: %w", err)
	}

	return tm, nil
}

// RefreshToken refreshes the GraphQL authentication token using Privy credentials
func (tm *TokenManager) RefreshToken() error {
	tm.logger.Println("Refreshing GraphQL authentication token...")

	// Try to get GraphQL token with current Privy tokens
	err := tm.refreshGraphQLToken()
	if err != nil {
		tm.logger.Printf("Failed to refresh GraphQL token: %v. Trying to refresh Privy tokens...", err)

		// If GraphQL refresh fails, try refreshing Privy tokens first
		err = tm.refreshPrivyTokens()
		if err != nil {
			return fmt.Errorf("failed to refresh Privy tokens: %w", err)
		}

		// Try GraphQL refresh again with new Privy tokens
		err = tm.refreshGraphQLToken()
		if err != nil {
			return fmt.Errorf("failed to refresh GraphQL token with new Privy tokens: %w", err)
		}
	}

	return nil
}

// refreshGraphQLToken refreshes just the GraphQL token using current Privy tokens
func (tm *TokenManager) refreshGraphQLToken() error {
	// Prepare GraphQL query for the loginWithPrivy mutation
	query := `
    mutation LoginWithPrivy {
      loginWithPrivy {
        token
      }
    }
  `

	// Build the request payload
	payload := map[string]string{
		"query":         query,
		"operationName": "LoginWithPrivy",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal login payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", graphqlEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("privy-authentication", tm.privyConfig.Authentication)
	req.Header.Set("privy-token", tm.privyConfig.Token)
	req.Header.Set("Origin", "https://boop.fun")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		return fmt.Errorf("login request failed with status %s: %s", resp.Status, bodyBytes.String())
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	// Check if the response contains a valid token
	if tokenResp.Data.LoginWithPrivy.Token == "" {
		return fmt.Errorf("received empty token in response")
	}

	// Update token
	tm.graphqlToken = tokenResp.Data.LoginWithPrivy.Token

	tm.logger.Println("Successfully refreshed GraphQL authentication token")
	return nil
}

// refreshPrivyTokens refreshes the Privy authentication tokens
func (tm *TokenManager) refreshPrivyTokens() error {
	tm.logger.Println("Refreshing Privy authentication tokens...")

	// Prepare request payload with refresh token
	payload := map[string]string{
		"refresh_token": tm.privyConfig.RefreshToken,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Privy refresh token payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", privyEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create Privy refresh request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	// Extract authentication token from the full Bearer string
	authToken := tm.privyConfig.Authentication
	if strings.HasPrefix(authToken, "Bearer ") {
		authToken = strings.TrimPrefix(authToken, "Bearer ")
	}
	req.Header.Set("authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("privy-app-id", tm.privyConfig.PrivyAppID)
	req.Header.Set("privy-ca-id", tm.privyConfig.PrivyClientID)
	req.Header.Set("privy-client", tm.privyConfig.PrivyClient)
	req.Header.Set("referer", "https://boop.fun/")
	req.Header.Set("Origin", "https://boop.fun")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Privy refresh request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		return fmt.Errorf("Privy refresh request failed with status %s: %s", resp.Status, bodyBytes.String())
	}

	// Parse response
	var privyResp PrivyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&privyResp); err != nil {
		return fmt.Errorf("failed to decode Privy refresh response: %w", err)
	}

	// Update Privy tokens
	if privyResp.Token != "" {
		tm.privyConfig.Authentication = fmt.Sprintf("Bearer %s", privyResp.Token)
	}
	if privyResp.IdentityToken != "" {
		tm.privyConfig.Token = privyResp.IdentityToken
	}
	if privyResp.RefreshToken != "" {
		tm.privyConfig.RefreshToken = privyResp.RefreshToken
	}

	tm.logger.Println("Successfully refreshed Privy authentication tokens")
	return nil
}

// GetAuthorizationHeader returns the current authorization header value
func (tm *TokenManager) GetAuthorizationHeader() string {
	// Ensure we have a token
	if tm.graphqlToken == "" {
		tm.logger.Println("WARNING: No GraphQL token available, attempting to refresh")
		if err := tm.RefreshToken(); err != nil {
			tm.logger.Printf("ERROR: Failed to refresh token: %v", err)
			return ""
		}
	}

	return fmt.Sprintf("Bearer %s", tm.graphqlToken)
}

// GetPrivyTokens returns the current Privy tokens
func (tm *TokenManager) GetPrivyTokens() (auth string, token string, refreshToken string) {
	return tm.privyConfig.Authentication, tm.privyConfig.Token, tm.privyConfig.RefreshToken
}

// NeedsRefresh checks if we need to refresh the token
// Call this before making API requests to ensure we have a valid token
func (tm *TokenManager) NeedsRefresh() bool {
	return tm.graphqlToken == ""
}
