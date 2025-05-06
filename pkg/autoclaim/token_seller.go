package autoclaim

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go/rpc"

	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/jupiter"
	"boop-airdrop-redeemer/pkg/models"
	"boop-airdrop-redeemer/pkg/notifications"
	"boop-airdrop-redeemer/pkg/service"
	"boop-airdrop-redeemer/pkg/solana"
)

// TokenSeller handles selling tokens after they've been claimed
type TokenSeller struct {
	config         *config.Config
	swapService    *jupiter.SwapService
	priceService   *solana.PriceService
	solClient      *rpc.Client
	telegramClient *notifications.TelegramClient
	logger         *log.Logger
}

// NewTokenSeller creates a new token seller instance
func NewTokenSeller(
	cfg *config.Config,
	claimer *service.AirdropClaimer,
	telegramClient *notifications.TelegramClient,
	logger *log.Logger,
) *TokenSeller {
	return &TokenSeller{
		config:         cfg,
		swapService:    claimer.GetSwapService(),
		priceService:   claimer.GetPriceService(),
		solClient:      claimer.GetSolClient(),
		telegramClient: telegramClient,
		logger:         logger,
	}
}

// SellToken attempts to sell a token for SOL
func (ts *TokenSeller) SellToken(ctx context.Context, airdrop models.AirdropNode) error {
	// Parse token amount
	tokenAmount, err := strconv.ParseUint(airdrop.AmountLpt, 10, 64)
	if err != nil {
		ts.logger.Printf("Failed to parse token amount for selling %s: %v", airdrop.ID, err)
		return err
	}

	// Get USD value for logging
	usdValue, _ := strconv.ParseFloat(airdrop.AmountUsd, 64)

	// Attempt to sell the token
	ts.logger.Printf("Selling token %s (%s) worth $%.2f...",
		airdrop.ID, airdrop.Token.Symbol, usdValue)

	swapSig, err := ts.swapService.SwapTokenForSol(
		ctx,
		ts.config.WalletPrivateKey,
		airdrop.Token.Address,
		tokenAmount,
	)

	if err != nil {
		ts.handleSellError(airdrop, err, 1)
		return err
	}

	// Get transaction signature
	txHash := swapSig.String()

	// Give the transaction some time to settle
	time.Sleep(2 * time.Second)

	// Get actual swap fees and earnings from the transaction
	swapFees, swapEarnings, err := solana.GetTransactionFeesAndEarnings(ts.solClient, txHash, true)
	if err != nil {
		ts.logger.Printf("Warning: Failed to get transaction details: %v", err)
		// Continue even if we couldn't get transaction details
		// We'll just use an estimate in this case
		ts.handleSuccessfulSaleWithEstimate(airdrop, txHash, usdValue)
		return nil
	}

	// Convert from lamports to SOL (1 SOL = 1,000,000,000 lamports)
	feesInSol := float64(swapFees) / 1_000_000_000
	earningsInSol := float64(swapEarnings) / 1_000_000_000

	ts.logger.Printf("Transaction details - fees: %.6f SOL, earnings: %.6f SOL", feesInSol, earningsInSol)

	// Handle successful sale with real transaction data
	ts.handleSuccessfulSale(airdrop, txHash, earningsInSol, feesInSol, usdValue)
	return nil
}

// handleSellError processes errors during token selling
func (ts *TokenSeller) handleSellError(airdrop models.AirdropNode, err error, attemptCount int) {
	ts.logger.Printf("Failed to sell token %s: %v", airdrop.ID, err)

	// If Telegram is enabled, send error notification
	if ts.telegramClient != nil && ts.telegramClient.Enabled {
		errorMsg := err.Error()
		if len(errorMsg) > 100 {
			errorMsg = errorMsg[:100] + "..."
		}
		ts.telegramClient.SendTokenSaleErrorNotification(
			airdrop.Token.Name,
			airdrop.Token.Symbol,
			airdrop.AmountLpt,
			airdrop.AmountUsd,
			errorMsg,
			attemptCount,
		)
	}
}

// handleSuccessfulSaleWithEstimate handles successful sale using estimated values
func (ts *TokenSeller) handleSuccessfulSaleWithEstimate(airdrop models.AirdropNode, txHash string, usdValue float64) {
	// Estimate SOL received based on current SOL price
	estimatedSolReceived := 0.0
	if ts.priceService != nil {
		solPrice := ts.priceService.GetCurrentPrice()
		if solPrice > 0 {
			estimatedSolReceived = usdValue / solPrice
		}
	}

	// Estimate fees (typically around 0.000005 SOL)
	estimatedFees := 0.000005

	// Net profit is earnings minus fees
	// No need to calculate netProfit as we're passing the individual components to handleSuccessfulSale

	ts.handleSuccessfulSale(airdrop, txHash, estimatedSolReceived, estimatedFees, usdValue)
}

// handleSuccessfulSale processes successful token sales
func (ts *TokenSeller) handleSuccessfulSale(airdrop models.AirdropNode, txHash string, earningsInSol float64, feesInSol float64, usdValue float64) {
	// Calculate net profit (earnings - fees)
	netProfitSol := earningsInSol - feesInSol

	ts.logger.Printf("🎉 Successfully sold token %s for %.6f SOL (fees: %.6f SOL, net: %.6f SOL)! Transaction: %s",
		airdrop.Token.Name, earningsInSol, feesInSol, netProfitSol, txHash)

	// Get SOL price for USD conversion
	solPrice := 0.0
	if ts.priceService != nil {
		solPrice = ts.priceService.GetCurrentPrice()
	}

	// Calculate USD values
	earningsUsd := earningsInSol * solPrice
	feesUsd := feesInSol * solPrice
	netProfitUsd := netProfitSol * solPrice

	// Create profit summary
	profitSummary := &notifications.ProfitSummary{
		Last24h:       netProfitSol,       // Just this transaction for now
		LastWeek:      netProfitSol,       // Just this transaction for now
		ProjectedWeek: netProfitSol * 7.0, // Simple projection
	}

	// Log detailed profit information
	ts.logger.Printf("Profit details: Earnings: %.6f SOL ($%.2f), Fees: %.6f SOL ($%.2f), Net: %.6f SOL ($%.2f)",
		earningsInSol, earningsUsd, feesInSol, feesUsd, netProfitSol, netProfitUsd)

	// Send notification about successful sale
	if ts.telegramClient != nil && ts.telegramClient.Enabled {
		ts.telegramClient.SendTokenSoldNotification(
			airdrop.Token.Name,
			airdrop.Token.Symbol,
			airdrop.AmountLpt,
			fmt.Sprintf("%.6f", netProfitSol),
			profitSummary,
			solPrice,
			txHash,
		)
	}
}
