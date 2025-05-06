package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/jupiter"
	"boop-airdrop-redeemer/pkg/solana/associated_token_account_extended"
	"boop-airdrop-redeemer/pkg/solana/boop"

	sol "boop-airdrop-redeemer/pkg/solana"

	"boop-airdrop-redeemer/pkg/notifications"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
)

// Boop program addresses
var (
	BoopTokenDistributor   = solana.MustPublicKeyFromBase58("J7cV46t2BLkoHWvmrcG1nK3wgB2D1EmHLko29bEDbnpV")
	BoopMerkleDistribution = solana.MustPublicKeyFromBase58("boopEtkTLx8x8moK7mMBQZUfzaEiA96Qn7gQeNdcQMg")
)

// ClaimConfig holds configuration for the claim process
type ClaimConfig struct {
	AutoSellToSol bool // Whether to automatically sell claimed tokens for SOL
}

// DefaultClaimConfig provides default settings for claiming
var DefaultClaimConfig = ClaimConfig{
	AutoSellToSol: true,
}

// ClaimRequest represents a request to claim an airdrop
type ClaimRequest struct {
	AirdropID       string `json:"id"`
	Wallet          string `json:"wallet"`
	Signature       string `json:"signature"`
	SignatureFormat string `json:"signatureFormat"`
}

// ClaimResponse represents a response from the claim API
type ClaimResponse struct {
	Success bool   `json:"success"`
	TxHash  string `json:"txHash,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AirdropClaimer handles claiming airdrops via transaction
type AirdropClaimer struct {
	config         *config.Config
	store          AirdropStore
	logger         *log.Logger
	solClient      *rpc.Client
	swapSvc        *jupiter.SwapService
	telegramClient *notifications.TelegramClient
	statsRecorder  *sol.StatsRecorder
	priceService   *sol.PriceService
}

// NewAirdropClaimer creates a new claimer with the provided dependencies
func NewAirdropClaimer(store AirdropStore, cfg *config.Config, logger *log.Logger, telegramClient *notifications.TelegramClient) *AirdropClaimer {
	// Initialize Solana RPC client
	solClient := rpc.New(cfg.SolanaRpcURL)

	// Initialize Jupiter swap service
	swapSvc := jupiter.NewSwapService(solClient, logger)

	// Initialize stats recorder
	statsRecorder, err := sol.NewStatsRecorder(cfg.StatsDataDir)
	if err != nil {
		logger.Printf("WARNING: Failed to initialize stats recorder: %v", err)
	}

	// Initialize price service
	priceService := sol.NewPriceService(logger)
	priceService.Start()

	return &AirdropClaimer{
		config:         cfg,
		store:          store,
		logger:         logger,
		solClient:      solClient,
		swapSvc:        swapSvc,
		telegramClient: telegramClient,
		statsRecorder:  statsRecorder,
		priceService:   priceService,
	}
}

// ClaimAirdropByID claims an airdrop by its ID
func (c *AirdropClaimer) ClaimAirdropByID(ctx context.Context, airdropID string) (string, error) {
	return c.ClaimAirdropByIDWithConfig(ctx, airdropID, DefaultClaimConfig)
}

// ClaimAirdropByIDWithConfig claims an airdrop by its ID with specific configuration options
func (c *AirdropClaimer) ClaimAirdropByIDWithConfig(ctx context.Context, airdropID string, config ClaimConfig) (string, error) {
	// Get airdrop information from the store
	airdrop, err := c.store.ClaimAirdrop(airdropID)
	if err != nil {
		return "", fmt.Errorf("failed to get airdrop: %w", err)
	}

	// Check if the airdrop is already claimed
	if airdrop.ClaimedAt != nil {
		return "", fmt.Errorf("airdrop %s is already claimed", airdropID)
	}

	c.logger.Printf("Claiming airdrop: %s, Token: %s (%s), Amount: %s",
		airdrop.ID, airdrop.Token.Name, airdrop.Token.Symbol, airdrop.AmountLpt)

	// Load private key from config
	if c.config.WalletPrivateKey == "" {
		return "", fmt.Errorf("wallet private key not configured")
	}

	feePayer, err := solana.PrivateKeyFromBase58(c.config.WalletPrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	signers := []solana.PrivateKey{feePayer}

	block, err := sol.BlockhashCache.GetBlockhash(c.solClient)
	if err != nil {
		return "", fmt.Errorf("failed to get blockhash: %w", err)
	}

	tokenAddress := solana.MustPublicKeyFromBase58(airdrop.Token.Address)

	ata, _, err := solana.FindAssociatedTokenAddress(feePayer.PublicKey(), tokenAddress)
	if err != nil {
		return "", fmt.Errorf("failed to find associated token address: %w", err)
	}

	instrs := []solana.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(200000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(375000).Build(),
		associated_token_account_extended.NewCreateIdempotentInstruction(
			feePayer.PublicKey(),
			feePayer.PublicKey(),
			tokenAddress,
		).Build(),
	}

	tokenAmount, err := strconv.ParseUint(airdrop.AmountLpt, 10, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse token amount: %w", err)
	}

	proofs := [][]interface{}{}
	proofs = append(proofs, airdrop.Proofs...)
	proofBytes := [][32]uint8{}

	for _, proof := range proofs {
		// Create a fixed size array for the proof
		var fixed [32]uint8

		// JSON-decoded proofs will be float64 values, not bytes
		for i, val := range proof {
			if i >= 32 {
				break
			}

			// Convert float64 to uint8
			if floatVal, ok := val.(float64); ok {
				fixed[i] = uint8(floatVal)
			} else {
				return "", fmt.Errorf("invalid proof value type: %T, expected float64", val)
			}
		}

		proofBytes = append(proofBytes, fixed)
	}

	tokenDistributor, err := sol.FindMerkleDistributorPDA(BoopTokenDistributor, tokenAddress, BoopMerkleDistribution, 0)
	if err != nil {
		return "", fmt.Errorf("failed to find merkle distributor pda: %w", err)
	}

	claimStatus, err := sol.FindClaimStatusPDA(feePayer.PublicKey(), tokenDistributor, BoopMerkleDistribution)
	if err != nil {
		return "", fmt.Errorf("failed to find claim status pda: %w", err)
	}

	boopPool, err := sol.FindBoopPoolAddress(tokenAddress, tokenDistributor, true)
	if err != nil {
		return "", fmt.Errorf("failed to find boop pool address: %w", err)
	}

	// Create the claim instruction and call Build() to get the actual instruction
	newClaimInstruction := boop.NewNewClaimInstructionBuilder(
		tokenAmount,
		0,
		proofBytes,
		tokenDistributor,
		claimStatus,
		boopPool,
		ata,
		feePayer.PublicKey(),
	).Build()

	instrs = append(instrs, newClaimInstruction)

	c.logger.Printf("Creating transaction with %d instructions", len(instrs))

	tx, err := solana.NewTransaction(
		instrs,
		block.Block.Blockhash,
		solana.TransactionPayer(feePayer.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	if _, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			for _, payer := range signers {
				if payer.PublicKey().Equals(key) {
					return &payer
				}
			}
			return nil
		},
	); err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	sig, err := c.solClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight: true,
			//MaxRetries:    &maxRetries,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	c.logger.Printf("Claim transaction complete for airdrop %s. Signature: %s", airdropID, sig.String())

	// Record transaction fees
	if c.statsRecorder != nil {
		// Wait a moment for the transaction to be confirmed
		time.Sleep(5 * time.Second)

		fees, _, err := sol.GetTransactionFeesAndEarnings(c.solClient, sig.String(), false)
		if err != nil {
			c.logger.Printf("Warning: Failed to get transaction fees: %v", err)
		} else {
			c.logger.Printf("Transaction fees: %d lamports (%.5f SOL)", fees, float64(fees)/1_000_000_000)

			err = c.statsRecorder.RecordClaimStats(
				airdrop.Token.Symbol,
				airdrop.AmountLpt,
				fees,
				sig.String(),
			)
			if err != nil {
				c.logger.Printf("Warning: Failed to record claim stats: %v", err)
			} else {
				c.logger.Printf("Recorded claim statistics for airdrop %s", airdropID)
			}
		}
	}

	// Send Telegram notification for successful claim
	if c.telegramClient != nil {
		usdValue := airdrop.AmountUsd
		// Pass raw token amount directly to the notification function
		amount := airdrop.AmountLpt
		c.telegramClient.SendTokenClaimedNotification(
			airdrop.Token.Name,
			airdrop.Token.Symbol,
			amount,
			usdValue,
			sig.String(),
		)
	}

	// If auto-sell is enabled, sell the token for SOL
	if config.AutoSellToSol {
		// Wait a bit for the claim transaction to confirm
		time.Sleep(5 * time.Second)

		c.logger.Printf("Auto-selling claimed tokens for SOL...")

		// Store claim transaction fees for profit calculation
		var claimFees uint64 = 0
		if c.statsRecorder != nil {
			// Get transaction fees for the claim
			claimFeesFromTx, _, err := sol.GetTransactionFeesAndEarnings(c.solClient, sig.String(), false)
			if err == nil {
				claimFees = claimFeesFromTx
			}
		}

		// Perform the swap
		swapSig, err := c.swapSvc.SwapTokenForSol(ctx, c.config.WalletPrivateKey, airdrop.Token.Address, tokenAmount)
		if err != nil {
			c.logger.Printf("Warning: Failed to auto-sell tokens after all retry attempts: %v", err)

			// If Telegram is enabled, send error notification
			if c.telegramClient != nil && c.telegramClient.Enabled {
				// Pass raw token amount directly to the notification function
				amount := airdrop.AmountLpt

				// Create a more readable error message
				errorMsg := err.Error()
				if len(errorMsg) > 100 {
					errorMsg = errorMsg[:100] + "..."
				}

				c.telegramClient.SendTokenSaleErrorNotification(
					airdrop.Token.Name,
					airdrop.Token.Symbol,
					amount,
					airdrop.AmountUsd,
					errorMsg,
					10, // Max attempts
				)
			}
		} else {
			c.logger.Printf("🎉 Successfully sold tokens for SOL! Transaction: %s", swapSig.String())

			// Variables for profit calculation
			var swapFees, swapEarnings uint64 = 0, 0
			var netProfit float64 = 0.0

			// Record swap transaction statistics
			if c.statsRecorder != nil {
				// Wait a moment for the transaction to be confirmed
				time.Sleep(5 * time.Second)

				// Get fees and earnings from the transaction
				var err error
				swapFees, swapEarnings, err = sol.GetTransactionFeesAndEarnings(c.solClient, swapSig.String(), true)
				if err != nil {
					c.logger.Printf("Warning: Failed to get swap transaction fees and earnings: %v", err)
				} else {
					c.logger.Printf("Swap fees: %d lamports (%.5f SOL)", swapFees, float64(swapFees)/1_000_000_000)
					c.logger.Printf("Earnings: %d lamports (%.5f SOL)", swapEarnings, float64(swapEarnings)/1_000_000_000)

					err = c.statsRecorder.RecordSwapStats(
						airdrop.Token.Symbol,
						airdrop.AmountLpt,
						swapFees,
						swapEarnings,
						swapSig.String(),
					)
					if err != nil {
						c.logger.Printf("Warning: Failed to record swap stats: %v", err)
					} else {
						c.logger.Printf("Recorded swap statistics for token %s", airdrop.Token.Symbol)
					}

					// Calculate net profit (earnings - all fees)
					netProfit = c.statsRecorder.CalculateNetProfitFromClaimAndSwap(claimFees, swapFees, swapEarnings)
					c.logger.Printf("Net profit for transaction: %.5f SOL", netProfit)
				}
			}

			// Get profit summary for the notification
			var profitSummary *notifications.ProfitSummary
			solPrice := 0.0

			// Calculate profit stats if possible
			if c.statsRecorder != nil {
				stats, err := c.statsRecorder.GetProfitSummary()
				if err == nil {
					// Convert to notifications.ProfitSummary
					profitSummary = &notifications.ProfitSummary{
						Last24h:       stats.Last24h,
						LastWeek:      stats.LastWeek,
						ProjectedWeek: stats.ProjectedWeek,
					}
				}

				// Get SOL price
				if c.priceService != nil {
					solPrice = c.priceService.GetCurrentPrice()
					c.logger.Printf("Current SOL price: $%.2f", solPrice)
				}
			}

			// Send notification about successful sale
			if c.telegramClient != nil {
				// Pass raw token amount directly to the notification function
				amount := airdrop.AmountLpt
				c.telegramClient.SendTokenSoldNotification(
					airdrop.Token.Name,
					airdrop.Token.Symbol,
					amount,
					fmt.Sprintf("%.5f", netProfit),
					profitSummary,
					solPrice,
					swapSig.String(),
				)
			}
		}
	}

	return sig.String(), nil
}

// CleanUp performs cleanup when the claimer is no longer needed
func (c *AirdropClaimer) CleanUp() {
	if c.priceService != nil {
		c.priceService.Stop()
		c.logger.Println("Stopped price service")
	}
}

// GetSwapService returns the swap service instance
func (c *AirdropClaimer) GetSwapService() *jupiter.SwapService {
	return c.swapSvc
}

// GetPriceService returns the price service instance
func (c *AirdropClaimer) GetPriceService() *sol.PriceService {
	return c.priceService
}

// GetSolClient returns the Solana client instance
func (c *AirdropClaimer) GetSolClient() *rpc.Client {
	return c.solClient
}
