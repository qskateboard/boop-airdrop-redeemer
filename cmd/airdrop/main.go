package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"boop-airdrop-redeemer/pkg/api"
	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/service"
)

func main() {
	// Create logger
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("Starting Boop Airdrop Redeemer...")

	// Load configuration
	cfg := config.NewConfig()
	logger.Printf("Configured for wallet: %s", cfg.WalletAddress)
	logger.Printf("Check interval: %s", cfg.CheckInterval)

	// Create API client
	boopClient := api.NewBoopClient(cfg, logger)

	// Create airdrop store
	store := service.NewInMemoryAirdropStore()

	// Create airdrop monitor
	monitor := service.NewAirdropMonitor(boopClient, store, cfg, logger)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the monitor
	if err := monitor.Start(ctx); err != nil {
		logger.Fatalf("Failed to start airdrop monitor: %v", err)
	}

	// Set up graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-signalCh
	logger.Println("Shutdown signal received")

	// Stop the monitor
	monitor.Stop()
	logger.Println("Airdrop monitor stopped, goodbye!")
}
