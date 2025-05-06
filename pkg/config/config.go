package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go"
)

// Config holds all configuration parameters for the application
type Config struct {
	GraphQLURL          string
	WalletAddress       string
	AuthToken           string
	PrivyAuth           string // privy-authentication header value
	PrivyToken          string // privy-token header value
	PrivyRefreshToken   string // privy refresh token
	CheckInterval       time.Duration
	Debug               bool
	SolanaRpcURL        string
	WalletPrivateKey    string
	MinimumUsdThreshold float64
	TokenManager        *TokenManager
	TelegramBotToken    string
	TelegramChatID      string
	EnableTelegram      bool
	StatsDataDir        string // Directory to store transaction statistics
}

// NewConfig creates a new configuration with default values or from environment variables
func NewConfig() *Config {
	minUsdThreshold := getEnv("MINIMUM_USD_THRESHOLD", "0.15")
	minUsdThresholdFloat, err := strconv.ParseFloat(minUsdThreshold, 64)
	if err != nil {
		log.Fatalf("Failed to parse MINIMUM_USD_THRESHOLD: %v", err)
	}

	// Create a logger for the config
	logger := log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)

	config := &Config{
		GraphQLURL:          getEnv("BOOP_API_URL", "https://graphql-mainnet.boop.works/graphql"),
		WalletAddress:       getEnv("WALLET_ADDRESS", ""),
		AuthToken:           getEnv("AUTH_TOKEN", ""),
		PrivyAuth:           getEnv("PRIVY_AUTH", ""),
		PrivyToken:          getEnv("PRIVY_TOKEN", ""),
		PrivyRefreshToken:   getEnv("PRIVY_REFRESH_TOKEN", ""),
		CheckInterval:       parseEnvDuration("CHECK_INTERVAL", 1*time.Minute),
		Debug:               getEnvBool("DEBUG", false),
		SolanaRpcURL:        getEnv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		WalletPrivateKey:    getEnv("WALLET_PRIVATE_KEY", ""),
		MinimumUsdThreshold: minUsdThresholdFloat,
		TelegramBotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:      getEnv("TELEGRAM_CHAT_ID", ""),
		EnableTelegram:      getEnvBool("ENABLE_TELEGRAM", false),
		StatsDataDir:        getEnv("STATS_DATA_DIR", "./data/stats"),
	}

	// Initialize the token manager
	privyAuth := getEnv("PRIVY_AUTH", "")
	privyToken := getEnv("PRIVY_TOKEN", "")
	privyRefreshToken := getEnv("PRIVY_REFRESH_TOKEN", "")
	config.TokenManager = NewTokenManager(privyAuth, privyToken, privyRefreshToken, logger)

	return config
}

// NewConfigWithPrivateKey creates a new configuration and initializes tokens using only a wallet private key
func NewConfigWithPrivateKey(privateKeyBase58 string) (*Config, error) {
	logger := log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)

	// Get public key from private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// Create config with default values
	minUsdThreshold := getEnv("MINIMUM_USD_THRESHOLD", "0.15")
	minUsdThresholdFloat, err := strconv.ParseFloat(minUsdThreshold, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MINIMUM_USD_THRESHOLD: %v", err)
	}

	config := &Config{
		GraphQLURL:          getEnv("BOOP_API_URL", "https://graphql-mainnet.boop.works/graphql"),
		WalletAddress:       privateKey.PublicKey().String(),
		WalletPrivateKey:    privateKeyBase58,
		CheckInterval:       parseEnvDuration("CHECK_INTERVAL", 1*time.Minute),
		Debug:               getEnvBool("DEBUG", false),
		SolanaRpcURL:        getEnv("SOLANA_RPC_URL", "https://api.mainnet-beta.solana.com"),
		MinimumUsdThreshold: minUsdThresholdFloat,
		TelegramBotToken:    getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:      getEnv("TELEGRAM_CHAT_ID", ""),
		EnableTelegram:      getEnvBool("ENABLE_TELEGRAM", false),
		StatsDataDir:        getEnv("STATS_DATA_DIR", "./data/stats"),
	}

	// Initialize tokens using private key
	err = config.InitTokenManagerWithPrivateKey(privateKeyBase58, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tokens: %w", err)
	}

	return config, nil
}

// InitTokenManager initializes the token manager with the Privy authentication tokens
func (c *Config) InitTokenManager(logger *log.Logger) {
	c.TokenManager = NewTokenManager(c.PrivyAuth, c.PrivyToken, c.PrivyRefreshToken, logger)

	// Immediately refresh to get a valid token
	if err := c.TokenManager.RefreshToken(); err != nil {
		logger.Printf("WARNING: Failed to initialize token: %v", err)
	} else {
		// Update the auth token with the fresh one
		c.AuthToken = c.TokenManager.GetAuthorizationHeader()

		// Update the Privy tokens in case they were refreshed
		c.PrivyAuth, c.PrivyToken, c.PrivyRefreshToken = c.TokenManager.GetPrivyTokens()
	}
}

// InitTokenManagerWithPrivateKey initializes the token manager directly with a wallet private key
func (c *Config) InitTokenManagerWithPrivateKey(privateKey string, logger *log.Logger) error {
	if logger == nil {
		logger = log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)
	}

	tokenManager, err := NewTokenManagerWithPrivateKey(privateKey, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize token manager with private key: %w", err)
	}

	c.TokenManager = tokenManager

	// Update the tokens in config with the fresh ones
	c.AuthToken = tokenManager.GetAuthorizationHeader()
	c.PrivyAuth, c.PrivyToken, c.PrivyRefreshToken = tokenManager.GetPrivyTokens()

	logger.Println("Successfully initialized token manager with private key")
	return nil
}

// GetAuthToken returns the current auth token, refreshing it if necessary
func (c *Config) GetAuthToken() string {
	if c.TokenManager != nil {
		return c.TokenManager.GetAuthorizationHeader()
	}
	return c.AuthToken
}

// RefreshAuthToken forces a refresh of the auth token
func (c *Config) RefreshAuthToken() error {
	if c.TokenManager == nil {
		return nil
	}

	err := c.TokenManager.RefreshToken()
	if err != nil {
		return err
	}

	// Update the config with the new token
	c.AuthToken = c.TokenManager.GetAuthorizationHeader()

	// Update the Privy tokens in case they were refreshed
	c.PrivyAuth, c.PrivyToken, c.PrivyRefreshToken = c.TokenManager.GetPrivyTokens()

	return nil
}

// Helper functions for working with environment variables
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func parseEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
