package autoclaim

import (
	"log"
	"strconv"
	"time"

	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/models"
)

// DecisionMaker handles the logic for deciding when to claim airdrops
type DecisionMaker struct {
	config *config.Config
	logger *log.Logger
}

// NewDecisionMaker creates a new decision maker
func NewDecisionMaker(cfg *config.Config) *DecisionMaker {
	return &DecisionMaker{
		config: cfg,
		logger: log.New(log.Writer(), "DECISION-MAKER: ", log.LstdFlags),
	}
}

// ShouldClaim determines if an airdrop should be claimed based on various criteria
func (d *DecisionMaker) ShouldClaim(airdrop models.AirdropNode, priceInfo *TokenPriceInfo) bool {
	if priceInfo == nil {
		return false
	}

	// Parse USD value
	usdValue, err := strconv.ParseFloat(airdrop.AmountUsd, 64)
	if err != nil {
		d.logger.Printf("Failed to parse USD value for airdrop %s: %v", airdrop.ID, err)
		return false
	}

	// Check if token meets regular threshold
	if usdValue >= d.config.MinimumUsdThreshold {
		return true
	}

	// Check if token meets special criteria
	if usdValue > 0.07 { // > 7 cents
		// Check if price has been stable for at least 10 minutes
		stableTime := time.Since(priceInfo.LastChanged)
		observedTime := time.Since(priceInfo.FirstObserved)

		if stableTime > 10*time.Minute && observedTime > 10*time.Minute {
			d.logger.Printf("Token %s price stable at $%.2f for %.1f minutes (observed for %.1f minutes), will claim",
				airdrop.Token.Symbol, usdValue, stableTime.Minutes(), observedTime.Minutes())
			return true
		} else if usdValue > 0.07 && (stableTime > 5*time.Minute || observedTime > 5*time.Minute) {
			// Log but don't claim yet
			d.logger.Printf("Tracking token %s at $%.2f - stable for %.1f minutes (observed for %.1f minutes)",
				airdrop.Token.Symbol, usdValue, stableTime.Minutes(), observedTime.Minutes())
		}
	}

	return false
}

// ShouldSellDirectly determines if a token should be sold directly
// For tokens that have already been claimed but have stable prices
func (d *DecisionMaker) ShouldSellDirectly(airdrop models.AirdropNode, priceInfo *TokenPriceInfo) bool {
	if priceInfo == nil || airdrop.ClaimedAt == nil {
		return false
	}

	// Parse USD value
	usdValue, err := strconv.ParseFloat(airdrop.AmountUsd, 64)
	if err != nil {
		return false
	}

	// For tokens with significant value but minimal price changes over time
	if usdValue >= 0.10 {
		stableTime := time.Since(priceInfo.LastChanged)
		observedTime := time.Since(priceInfo.FirstObserved)

		// If price has been stable for extended period with minimal changes
		if stableTime >= 10*time.Minute && observedTime >= 10*time.Minute {
			return true
		}
	}

	return false
}
