package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"boop-airdrop-redeemer/pkg/api"
	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/models"
)

// AirdropScanner handles scanning for new airdrops
type AirdropScanner struct {
	client *api.BoopClient
	store  AirdropStore
	logger *log.Logger
}

// NewAirdropScanner creates a new scanner with the provided dependencies
func NewAirdropScanner(store AirdropStore, cfg *config.Config, logger *log.Logger) *AirdropScanner {
	// Initialize Boop API client
	client := api.NewBoopClient(cfg, logger)

	return &AirdropScanner{
		client: client,
		store:  store,
		logger: logger,
	}
}

// ScanAirdrops scans for all airdrops, includes previously seen airdrops but updates their values
// Returns all valuable airdrops that meet the threshold
func (s *AirdropScanner) ScanAirdrops(ctx context.Context, usdThreshold float64) ([]models.AirdropNode, error) {
	// Fetch pending airdrops from API
	allAirdrops, err := s.client.GetPendingAirdrops(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch airdrops: %w", err)
	}

	valuableAirdrops := []models.AirdropNode{}
	newAirdropCount := 0

	// Process all airdrops
	for _, airdrop := range allAirdrops {
		isNew := !s.store.HasAirdropWithID(airdrop.ID)
		if isNew {
			newAirdropCount++
		}

		// Save or update airdrop in store regardless if it's new or existing
		s.store.SaveAirdrop(airdrop)

		// Check airdrop value
		amountUsd, err := strconv.ParseFloat(airdrop.AmountUsd, 64)
		if err != nil {
			s.logger.Printf("Error parsing amount USD for airdrop %s: %v", airdrop.ID, err)
			continue
		}

		// Log differently for new vs. updated airdrops
		if isNew {
			s.logger.Printf("Found new airdrop: ID=%s, Token=%s (%s), Amount=$%.2f",
				airdrop.ID, airdrop.Token.Name, airdrop.Token.Symbol, amountUsd)
		} else {
			if amountUsd > 0.05 {
				s.logger.Printf("Updated airdrop price: ID=%s, Token=%s (%s), Current value=$%.2f",
					airdrop.ID, airdrop.Token.Name, airdrop.Token.Symbol, amountUsd)
			}
		}

		// Add to valuable airdrops if it meets threshold
		if amountUsd >= usdThreshold {
			valuableAirdrops = append(valuableAirdrops, airdrop)
		}
	}

	if newAirdropCount == 0 {
		s.logger.Println("No new airdrops found")
	} else {
		s.logger.Printf("Found %d new airdrop(s)", newAirdropCount)
	}

	if len(valuableAirdrops) > 0 {
		s.logger.Printf("Found %d airdrop(s) meeting value threshold", len(valuableAirdrops))
	}

	return valuableAirdrops, nil
}

// ScanNewAirdrops scans for new airdrops and returns them (for backward compatibility)
func (s *AirdropScanner) ScanNewAirdrops(ctx context.Context) ([]models.AirdropNode, error) {
	// Fetch pending airdrops from API
	allAirdrops, err := s.client.GetPendingAirdrops(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch airdrops: %w", err)
	}

	// Filter out airdrops that are already known
	newAirdrops := []models.AirdropNode{}
	for _, airdrop := range allAirdrops {
		// Check if we already know about this airdrop
		if !s.store.HasAirdropWithID(airdrop.ID) {
			// This is a new airdrop
			amountUsd, err := strconv.ParseFloat(airdrop.AmountUsd, 64)
			if err != nil {
				s.logger.Printf("Error parsing amount usd for airdrop %s: %v", airdrop.ID, err)
				continue
			}

			s.logger.Printf("Found new airdrop: ID=%s, Token=%s (%s), Amount=$%.2f",
				airdrop.ID, airdrop.Token.Name, airdrop.Token.Symbol, amountUsd)

			// Save it to the store
			s.store.SaveAirdrop(airdrop)

			// Add to the list of new airdrops
			newAirdrops = append(newAirdrops, airdrop)
		}
	}

	if len(newAirdrops) == 0 {
		s.logger.Println("No new airdrops found")
	} else {
		s.logger.Printf("Found %d new airdrop(s)", len(newAirdrops))
	}

	return newAirdrops, nil
}
