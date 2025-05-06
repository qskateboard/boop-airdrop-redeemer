package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gagliardetto/solana-go"
)

const (
	privyInitURL         = "https://auth.privy.io/api/v1/siws/init"
	privyAuthenticateURL = "https://auth.privy.io/api/v1/siws/authenticate"
)

// PrivyConfig for the headers used in privy requests
type PrivyRequestConfig struct {
	AppID     string
	ClientID  string
	Client    string
	UserAgent string
}

// DefaultPrivyConfig provides default Privy API request configuration
var DefaultPrivyConfig = PrivyRequestConfig{
	AppID:     "cm9qu1hed02wwl50m7cd5396n",
	ClientID:  "eea5a712-be5e-4965-aceb-9e9a77db3492",
	Client:    "react-auth:2.13.0-beta-20250501014923",
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
}

// PrivyInitResponse represents the response from the init API
type PrivyInitResponse struct {
	Nonce     string `json:"nonce"`
	Address   string `json:"address"`
	ExpiresAt string `json:"expires_at"`
}

// PrivyAuthResponse represents the response from the authentication API
type PrivyAuthResponse struct {
	User struct {
		ID             string                   `json:"id"`
		CreatedAt      int64                    `json:"created_at"`
		LinkedAccounts []map[string]interface{} `json:"linked_accounts"`
	} `json:"user"`
	Token            string `json:"token"`
	PrivyAccessToken string `json:"privy_access_token"`
	RefreshToken     string `json:"refresh_token"`
	IdentityToken    string `json:"identity_token"`
}

// GetPrivyTokensWithPrivateKey obtains Privy authentication tokens using a wallet private key
func GetPrivyTokensWithPrivateKey(privateKeyBase58 string, logger *log.Logger) (string, string, string, error) {
	if logger == nil {
		logger = log.New(log.Writer(), "[PRIVY AUTH] ", log.LstdFlags)
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid private key: %w", err)
	}

	// Get public key from private key
	publicKey := privateKey.PublicKey()
	walletAddress := publicKey.String()

	logger.Printf("Authenticating with Privy for wallet: %s", walletAddress)

	// Step 1: Initialize SIWS (Sign In With Solana) process
	nonce, err := initPrivySignIn(walletAddress)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to initialize Privy sign-in: %w", err)
	}

	logger.Printf("Received nonce: %s", nonce)

	// Step 2: Generate the message to sign
	message := generatePrivySignMessage(walletAddress, nonce)
	logger.Printf("Generated message to sign: %s", message)

	// Step 3: Sign the message
	signature, err := signPrivyMessage(privateKey, message)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to sign message: %w", err)
	}

	// Step 4: Authenticate with the signed message
	authResponse, err := authenticateWithPrivy(walletAddress, message, signature)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to authenticate with Privy: %w", err)
	}

	token := fmt.Sprintf("Bearer %s", authResponse.Token)

	logger.Printf("Successfully authenticated with Privy")

	// Return the authentication token, identity token, and refresh token
	return token, authResponse.IdentityToken, authResponse.RefreshToken, nil
}

// initPrivySignIn initializes the Sign In With Solana process
func initPrivySignIn(walletAddress string) (string, error) {
	// Create the payload
	payload := map[string]string{
		"address": walletAddress,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", privyInitURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	setPrivyHeaders(req)

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send init request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		return "", fmt.Errorf("init request failed with status %s: %s", resp.Status, bodyBytes.String())
	}

	// Parse response
	var initResp PrivyInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return "", fmt.Errorf("failed to decode init response: %w", err)
	}

	return initResp.Nonce, nil
}

// generatePrivySignMessage generates the message to sign for Privy authentication
func generatePrivySignMessage(walletAddress, nonce string) string {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.999Z")

	return fmt.Sprintf(
		"boop.fun wants you to sign in with your Solana account:\n"+
			"%s\n\n"+
			"You are proving you own %s.\n\n"+
			"URI: %s\n"+
			"Version: %s\n"+
			"Chain ID: %s\n"+
			"Nonce: %s\n"+
			"Issued At: %s\n"+
			"Resources:\n- %s",
		walletAddress,
		walletAddress,
		"https://boop.fun",
		"1",
		"mainnet",
		nonce,
		now,
		"https://privy.io",
	)
}

// signPrivyMessage signs a message with a private key
func signPrivyMessage(privateKey solana.PrivateKey, message string) (string, error) {
	// Sign the message using Ed25519
	signature, err := privateKey.Sign([]byte(message))

	if err != nil {
		return "", fmt.Errorf("failed to sign message: %w", err)
	}

	// Encode the signature as base64
	return base64.StdEncoding.EncodeToString([]byte(signature[:])), nil
}

// authenticateWithPrivy completes authentication with the signed message
func authenticateWithPrivy(walletAddress, message, signature string) (*PrivyAuthResponse, error) {
	// Create the payload
	payload := map[string]interface{}{
		"message":          message,
		"signature":        signature,
		"walletClientType": "phantom",
		"connectorType":    "solana_adapter",
		"mode":             "login-or-sign-up",
		"message_type":     "plain",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", privyAuthenticateURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set headers
	setPrivyHeaders(req)

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send auth request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		return nil, fmt.Errorf("auth request failed with status %s: %s", resp.Status, bodyBytes.String())
	}

	// Parse response
	var authResp PrivyAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %w", err)
	}

	return &authResp, nil
}

// setPrivyHeaders sets the required headers for Privy API requests
func setPrivyHeaders(req *http.Request) {
	config := DefaultPrivyConfig

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://boop.fun")
	req.Header.Set("Referer", "https://boop.fun/")
	req.Header.Set("privy-app-id", config.AppID)
	req.Header.Set("privy-ca-id", config.ClientID)
	req.Header.Set("privy-client", config.Client)
	req.Header.Set("User-Agent", config.UserAgent)
}
