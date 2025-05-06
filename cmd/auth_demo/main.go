package main

import (
	"fmt"
	"log"
	"os"

	"boop-airdrop-redeemer/pkg/config"
)

func main() {
	// Check if private key is provided as argument
	if len(os.Args) < 2 {
		fmt.Println("Usage: auth_demo <private-key>")
		os.Exit(1)
	}

	privateKey := os.Args[1]
	logger := log.New(os.Stdout, "AUTH DEMO: ", log.LstdFlags)

	// Method 1: Using direct private key to tokens conversion
	logger.Println("Method 1: Direct private key to tokens conversion")
	privyAuth, privyToken, privyRefresh, err := config.GetPrivyTokensWithPrivateKey(privateKey, logger)
	if err != nil {
		logger.Fatalf("Failed to authenticate: %v", err)
	}

	logger.Println("Successfully obtained Privy tokens")
	logger.Printf("Auth: %s", truncateString(privyAuth, 15))
	logger.Printf("Token: %s", truncateString(privyToken, 15))
	logger.Printf("Refresh: %s", truncateString(privyRefresh, 15))

	// Method 2: Using token manager
	logger.Println("\nMethod 2: Using TokenManager")
	tokenManager, err := config.NewTokenManagerWithPrivateKey(privateKey, logger)
	if err != nil {
		logger.Fatalf("Failed to create token manager: %v", err)
	}

	graphqlToken := tokenManager.GetAuthorizationHeader()
	logger.Printf("GraphQL Token: %s", truncateString(graphqlToken, 15))

	// Method 3: Full configuration setup
	logger.Println("\nMethod 3: Full configuration with private key")
	cfg, err := config.NewConfigWithPrivateKey(privateKey)
	if err != nil {
		logger.Fatalf("Failed to create config: %v", err)
	}

	logger.Printf("Config initialized for wallet: %s", cfg.WalletAddress)
	logger.Printf("GraphQL Auth Token: %s", truncateString(cfg.AuthToken, 15))

	logger.Println("\nAuth demo completed successfully!")
}

// truncateString truncates a string and adds "..." if it exceeds the given length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
