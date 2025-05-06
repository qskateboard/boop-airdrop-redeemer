package autoclaim

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/models"
	"boop-airdrop-redeemer/pkg/notifications"
	"boop-airdrop-redeemer/pkg/service"
)

// Service handles the orchestration of auto claiming airdrops
type Service struct {
	config         *config.Config
	scanner        *service.AirdropScanner
	claimer        *service.AirdropClaimer
	priceTracker   *PriceTracker
	decisionMaker  *DecisionMaker
	tokenSeller    *TokenSeller
	telegramClient *notifications.TelegramClient
	logger         *log.Logger

	// Track claimed airdrops
	claimedAirdrops map[string]bool
	claimedMutex    *sync.Mutex

	// Track auth token refresh
	lastTokenRefresh time.Time
}

// NewService creates a new auto claim service
func NewService(
	cfg *config.Config,
	scanner *service.AirdropScanner,
	claimer *service.AirdropClaimer,
	telegramClient *notifications.TelegramClient,
	logger *log.Logger,
) *Service {
	return &Service{
		config:           cfg,
		scanner:          scanner,
		claimer:          claimer,
		telegramClient:   telegramClient,
		logger:           logger,
		priceTracker:     NewPriceTracker(),
		decisionMaker:    NewDecisionMaker(cfg),
		tokenSeller:      NewTokenSeller(cfg, claimer, telegramClient, logger),
		claimedAirdrops:  make(map[string]bool),
		claimedMutex:     &sync.Mutex{},
		lastTokenRefresh: time.Time{}, // Zero time
	}
}

// Start begins the auto claiming service
func (s *Service) Start(ctx context.Context) {
	if s.telegramClient.Enabled {
		// Send welcome message with bot information and settings
		s.telegramClient.SendWelcomeMessage(
			s.config.WalletAddress,
			s.config.MinimumUsdThreshold,
			s.config.CheckInterval,
		)
	}

	// Run in a loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
			s.processAirdrops(ctx)

			// Wait before the next scan
			s.logger.Println("Waiting for next scan cycle...")
			time.Sleep(s.config.CheckInterval)
		}
	}
}

// processAirdrops scans for and processes available airdrops
func (s *Service) processAirdrops(ctx context.Context) {
	// Scan for airdrops (including previously seen ones to update values)
	s.logger.Println("Scanning for airdrops and updating values...")
	valuableAirdrops, err := s.scanner.ScanAirdrops(ctx, 0.001) // Use a very low threshold to get all airdrops
	if err != nil {
		// Check if error is related to authentication
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
			strings.Contains(strings.ToLower(err.Error()), "auth") ||
			strings.Contains(strings.ToLower(err.Error()), "token") {
			// Refresh token on auth errors
			s.refreshAuthToken()
		}

		s.handleScanError(err)
		return
	}

	// Update price history and find claimable airdrops
	filteredAirdrops := s.processAndFilterAirdrops(ctx, valuableAirdrops)
	s.logger.Printf("Found %d valuable airdrop(s) meeting threshold", len(filteredAirdrops))

	// Process each valuable airdrop
	for _, airdrop := range filteredAirdrops {
		if s.isAlreadyClaimed(airdrop) {
			continue
		}

		// Attempt to claim the airdrop
		txHash, err := s.claimer.ClaimAirdropByID(ctx, airdrop.ID)
		if err != nil {
			// If error is auth-related, try refreshing the token and retry once
			if strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
				strings.Contains(strings.ToLower(err.Error()), "auth") ||
				strings.Contains(strings.ToLower(err.Error()), "token") {
				s.refreshAuthToken()
				// Retry the claim after token refresh
				txHash, err = s.claimer.ClaimAirdropByID(ctx, airdrop.ID)
			}
		}

		s.handleClaimResult(ctx, airdrop, txHash, err)

		// Wait between claims to avoid transaction failures
		s.logger.Println("Waiting 60 seconds before checking for more airdrops...")
		time.Sleep(60 * time.Second)
		break
	}
}

// refreshAuthToken refreshes the authentication token if using private key auth
// and it hasn't been refreshed recently
func (s *Service) refreshAuthToken() {
	// Only proceed if using private key auth and token manager exists
	if s.config.TokenManager == nil || s.config.WalletPrivateKey == "" {
		return
	}

	// Check if we've refreshed recently (within last 30 minutes)
	if !s.lastTokenRefresh.IsZero() && time.Since(s.lastTokenRefresh) < 30*time.Minute {
		s.logger.Println("Token was refreshed recently, skipping refresh")
		return
	}

	// Refresh the token
	s.logger.Println("Refreshing authentication token...")
	err := s.config.RefreshAuthToken()
	if err != nil {
		s.logger.Printf("Warning: Failed to refresh auth token: %v", err)
	} else {
		s.lastTokenRefresh = time.Now()
		s.logger.Println("Auth token refreshed successfully")
	}
}

// handleScanError processes errors during airdrop scanning
func (s *Service) handleScanError(err error) {
	s.logger.Printf("Error scanning airdrops: %v", err)

	// Check for network error and retry quickly
	if strings.Contains(err.Error(), "unexpected EOF") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "i/o timeout") {
		s.logger.Println("Network error detected, retrying in 3 seconds...")
		time.Sleep(3 * time.Second)
		return
	}

	time.Sleep(30 * time.Second)
}

// handleClaimResult processes the result of a claim attempt
func (s *Service) handleClaimResult(ctx context.Context, airdrop models.AirdropNode, txHash string, err error) {
	usdValue, _ := strconv.ParseFloat(airdrop.AmountUsd, 64)

	if err != nil {
		s.logger.Printf("Failed to claim airdrop %s: %v", airdrop.ID, err)

		// If it's a permanent error, mark as claimed to prevent repeated attempts
		if IsPermanentClaimError(err) {
			s.logger.Printf("Marking airdrop %s as claimed due to permanent error", airdrop.ID)
			s.claimedMutex.Lock()
			s.claimedAirdrops[airdrop.ID] = true
			s.claimedMutex.Unlock()
		}
		return
	}

	// Mark as successfully claimed
	s.logger.Printf("Successfully claimed airdrop %s (%s) worth $%.2f! Transaction hash: %s",
		airdrop.ID, airdrop.Token.Symbol, usdValue, txHash)
	s.claimedMutex.Lock()
	s.claimedAirdrops[airdrop.ID] = true
	s.claimedMutex.Unlock()
}

// processAndFilterAirdrops processes all airdrops and returns those that should be claimed
func (s *Service) processAndFilterAirdrops(ctx context.Context, airdrops []models.AirdropNode) []models.AirdropNode {
	var filteredAirdrops []models.AirdropNode

	for _, airdrop := range airdrops {
		// Skip if already claimed
		if s.isAlreadyClaimed(airdrop) {
			continue
		}

		// Update price tracking data
		s.priceTracker.UpdatePriceData(airdrop)

		// Check if we should claim this airdrop
		priceInfo := s.priceTracker.GetTokenPriceInfo(airdrop.ID)
		if s.decisionMaker.ShouldClaim(airdrop, priceInfo) {
			filteredAirdrops = append(filteredAirdrops, airdrop)
		} else {
			// Check if stable token should be sold directly
			s.handleStableTokenSale(ctx, airdrop, priceInfo)
		}
	}

	return filteredAirdrops
}

// isAlreadyClaimed checks if an airdrop has already been claimed
func (s *Service) isAlreadyClaimed(airdrop models.AirdropNode) bool {
	s.claimedMutex.Lock()
	defer s.claimedMutex.Unlock()

	if s.claimedAirdrops[airdrop.ID] {
		s.logger.Printf("Skipping already claimed airdrop: %s (%s)",
			airdrop.ID, airdrop.Token.Symbol)
		return true
	}

	// Check if airdrop is already claimed according to the data
	if airdrop.ClaimedAt != nil {
		s.logger.Printf("Airdrop %s is already claimed", airdrop.ID)
		s.claimedAirdrops[airdrop.ID] = true
		return true
	}

	return false
}

// claimAirdrop attempts to claim an airdrop
func (s *Service) claimAirdrop(ctx context.Context, airdrop models.AirdropNode) {
	usdValue, _ := strconv.ParseFloat(airdrop.AmountUsd, 64)
	s.logger.Printf("Claiming airdrop %s (%s) worth $%.2f...",
		airdrop.ID, airdrop.Token.Symbol, usdValue)

	txHash, err := s.claimer.ClaimAirdropByID(ctx, airdrop.ID)
	if err != nil {
		s.logger.Printf("Failed to claim airdrop %s: %v", airdrop.ID, err)

		// If it's a permanent error, mark as claimed to prevent repeated attempts
		if IsPermanentClaimError(err) {
			s.logger.Printf("Marking airdrop %s as claimed due to permanent error", airdrop.ID)
			s.claimedMutex.Lock()
			s.claimedAirdrops[airdrop.ID] = true
			s.claimedMutex.Unlock()
		}
		return
	}

	// Mark as successfully claimed
	s.logger.Printf("Successfully claimed airdrop %s! Transaction hash: %s", airdrop.ID, txHash)
	s.claimedMutex.Lock()
	s.claimedAirdrops[airdrop.ID] = true
	s.claimedMutex.Unlock()
}

// handleStableTokenSale handles selling tokens that are already claimed and stable in price
func (s *Service) handleStableTokenSale(ctx context.Context, airdrop models.AirdropNode, priceInfo *TokenPriceInfo) {
	if airdrop.ClaimedAt == nil || priceInfo == nil {
		return
	}

	// Check if this token should be sold
	if s.decisionMaker.ShouldSellDirectly(airdrop, priceInfo) {
		// Check if already claimed by us (to avoid double handling)
		s.claimedMutex.Lock()
		isClaimed := s.claimedAirdrops[airdrop.ID]
		s.claimedMutex.Unlock()

		if !isClaimed {
			usdValue, _ := strconv.ParseFloat(airdrop.AmountUsd, 64)
			s.logger.Printf("Token %s (%s) has stable price at $%.2f - attempting to sell directly",
				airdrop.Token.Name, airdrop.Token.Symbol, usdValue)

			// Sell token in a goroutine to not block the main process
			go func(airdropCopy models.AirdropNode) {
				err := s.tokenSeller.SellToken(context.Background(), airdropCopy)
				if err == nil {
					// Mark as claimed/sold to prevent future attempts
					s.claimedMutex.Lock()
					s.claimedAirdrops[airdropCopy.ID] = true
					s.claimedMutex.Unlock()
				} else if strings.Contains(strings.ToLower(err.Error()), "unauthorized") ||
					strings.Contains(strings.ToLower(err.Error()), "auth") ||
					strings.Contains(strings.ToLower(err.Error()), "token") {
					// Refresh token on auth errors during sale
					s.refreshAuthToken()
				}
			}(airdrop)
		}
	}
}

// IsPermanentClaimError determines if an error during claiming is permanent and not worth retrying
func IsPermanentClaimError(err error) bool {
	errorMsg := err.Error()

	// Check for common permanent errors
	permanentErrors := []string{
		"airdrop is already claimed",
		"invalid token amount",
		"failed to parse token amount",
		"failed to find merkle distributor",
	}

	for _, msg := range permanentErrors {
		if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(msg)) {
			return true
		}
	}

	return false
}
