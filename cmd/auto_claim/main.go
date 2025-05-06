package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boop-airdrop-redeemer/pkg/autoclaim"
	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/notifications"
	"boop-airdrop-redeemer/pkg/service"
)

func main() {
	logger := log.New(os.Stdout, "AUTO-CLAIMER: ", log.LstdFlags)
	logger.Println("Starting Boop Auto Claimer Service...")

	// Load configuration
	var cfg *config.Config
	var err error

	// Check if wallet private key is provided in environment
	privateKey := os.Getenv("WALLET_PRIVATE_KEY")
	if privateKey != "" {
		logger.Println("Private key found, initializing with private key authentication...")
		cfg, err = config.NewConfigWithPrivateKey(privateKey)
		if err != nil {
			logger.Fatalf("Failed to initialize with private key: %v", err)
		}
		logger.Println("Successfully authenticated using wallet private key")
	} else {
		// Fall back to traditional auth token method
		cfg = config.NewConfig()
	}

	logger.Printf("Configured for wallet: %s", cfg.WalletAddress)
	logger.Printf("Using minimum value threshold: $%.2f", cfg.MinimumUsdThreshold)

	// Create Telegram notification client
	telegramClient := notifications.NewTelegramClient(
		cfg.TelegramBotToken,
		cfg.TelegramChatID,
		cfg.EnableTelegram,
	)

	// Create scanner and claimer services
	store := service.NewInMemoryAirdropStore()
	scanner := service.NewAirdropScanner(store, cfg, logger)
	claimer := service.NewAirdropClaimer(store, cfg, logger, telegramClient)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create auto claimer service
	autoClaimService := autoclaim.NewService(cfg, scanner, claimer, telegramClient, logger)

	// Run the auto claimer in a goroutine
	go autoClaimService.Start(ctx)

	// Wait for termination signal
	<-sigChan
	logger.Println("Received termination signal. Shutting down...")
	cancel()

	// Clean up resources
	claimer.CleanUp()
	logger.Println("Resources cleaned up")

	time.Sleep(2 * time.Second) // Give time for cleanup
}
