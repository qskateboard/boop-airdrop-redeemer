package jupiter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	sln "boop-airdrop-redeemer/pkg/solana"
)

// SwapService handles Jupiter swap operations
type SwapService struct {
	client    *Client
	solClient *rpc.Client
	logger    *log.Logger
}

// NewSwapService creates a new swap service
func NewSwapService(solClient *rpc.Client, logger *log.Logger) *SwapService {
	return &SwapService{
		client:    NewClient(logger),
		solClient: solClient,
		logger:    logger,
	}
}

// GetWallet retrieves the wallet from the private key
func (s *SwapService) GetWallet(privateKeyBase58 string) (solana.PrivateKey, error) {
	if privateKeyBase58 == "" {
		return solana.PrivateKey{}, fmt.Errorf("private key not provided")
	}
	wallet, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		return solana.PrivateKey{}, fmt.Errorf("invalid private key: %w", err)
	}
	return wallet, nil
}

// GetTokenBalances fetches SPL token balances for the wallet
func (s *SwapService) GetTokenBalances(ctx context.Context, owner solana.PublicKey) (map[string]TokenBalance, error) {
	// Get all token accounts for the owner
	accounts, err := s.solClient.GetTokenAccountsByOwner(
		ctx,
		owner,
		&rpc.GetTokenAccountsConfig{
			ProgramId: &solana.TokenProgramID, // Filter by SPL Token Program
		},
		&rpc.GetTokenAccountsOpts{
			Encoding: solana.EncodingJSONParsed,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get token accounts: %w", err)
	}

	// Map to store token balances with USD values
	balances := make(map[string]TokenBalance)

	// Extract mints to get prices
	mints := []string{}

	// Process token accounts
	for _, account := range accounts.Value {
		// We need to process the parsed JSON account data manually
		if account.Account.Data.GetRawJSON() == nil {
			s.logger.Printf("⚠️ Skipping account %s: not JSON parsed data", account.Pubkey)
			continue
		}

		// Extract data from the JSON object
		data := account.Account.Data.GetRawJSON()
		var parsedData map[string]interface{}
		err = json.Unmarshal(data, &parsedData)
		if err != nil {
			s.logger.Printf("⚠️ Failed to unmarshal account data: %v", err)
			continue
		}
		if parsedData["parsed"] == nil {
			continue
		}

		parsed, ok := parsedData["parsed"].(map[string]interface{})
		if !ok || parsed["info"] == nil {
			continue
		}

		info, ok := parsed["info"].(map[string]interface{})
		if !ok || info["tokenAmount"] == nil || info["mint"] == nil {
			continue
		}

		// Get the token mint address
		tokenMint, ok := info["mint"].(string)
		if !ok {
			continue
		}

		// Get token amount details
		tokenAmount, ok := info["tokenAmount"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract amount values
		amount, _ := tokenAmount["amount"].(string)
		uiAmountStr, _ := tokenAmount["uiAmountString"].(string)
		decimalsFloat, _ := tokenAmount["decimals"].(float64)
		decimals := uint8(decimalsFloat)

		// Parse amount values
		rawAmount, _ := strconv.ParseUint(amount, 10, 64)
		uiAmount, _ := strconv.ParseFloat(uiAmountStr, 64)

		// Only process tokens with positive balance
		if rawAmount > 0 {
			mints = append(mints, tokenMint)

			// Initialize balance data
			balances[tokenMint] = TokenBalance{
				Mint:     tokenMint,
				Amount:   rawAmount,
				Decimals: decimals,
				UiAmount: uiAmount,
			}

			s.logger.Printf("  - Found: %s, Amount: %s (Raw: %d), Decimals: %d",
				tokenMint, uiAmountStr, rawAmount, decimals)
		}
	}

	// Get prices for all tokens
	if len(mints) > 0 {
		prices, err := s.client.GetPrices(mints)
		if err != nil {
			s.logger.Printf("Warning: failed to get token prices: %v", err)
		} else {
			// Update balances with price information
			for mint, price := range prices {
				if balance, exists := balances[mint]; exists {
					balance.UsdPrice = price
					balance.UsdValue = balance.UiAmount * price
					balances[mint] = balance

					s.logger.Printf("  - Price for %s: $%.4f, Total value: $%.2f",
						mint, price, balance.UsdValue)
				}
			}
		}
	}

	return balances, nil
}

// SwapTokenForUsdc swaps a token for USDC
func (s *SwapService) SwapTokenForUsdc(ctx context.Context, privateKeyBase58 string, inputMint string, amount uint64) (solana.Signature, error) {
	// Get wallet
	wallet, err := s.GetWallet(privateKeyBase58)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get wallet: %w", err)
	}

	// Get public key
	pubKey := wallet.PublicKey()

	// Step 1: Get quote
	s.logger.Printf("Getting swap quote for %d units of %s -> USDC...", amount, inputMint)
	quote, err := s.client.GetSwapQuote(inputMint, QuoteCurrencyMint, amount)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get swap quote: %w", err)
	}

	// Calculate output amount in USDC (with 6 decimals)
	outAmountRaw, _ := strconv.ParseUint(quote.OutAmount, 10, 64)
	outAmountFormatted := float64(outAmountRaw) / math.Pow10(6) // USDC has 6 decimals

	s.logger.Printf("Got quote - Will receive: %.2f USDC", outAmountFormatted)

	// Step 2: Get transaction
	s.logger.Printf("Getting swap transaction...")
	swapResp, err := s.client.GetSwapTransaction(quote, pubKey)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get swap transaction: %w", err)
	}

	// Step 3: Sign and send transaction
	s.logger.Printf("Signing and sending transaction...")
	sig, err := s.client.SignAndSendTransaction(ctx, s.solClient, swapResp.SwapTransaction, wallet)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to sign and send transaction: %w", err)
	}

	s.logger.Printf("🎉 Swap transaction sent! Signature: %s", sig.String())
	s.logger.Printf("View on Solscan: https://solscan.io/tx/%s", sig.String())

	return sig, nil
}

// GetTokensToSell identifies tokens that meet the threshold for selling
func (s *SwapService) GetTokensToSell(ctx context.Context, owner solana.PublicKey, minimumUsdThreshold float64) ([]TokenBalance, error) {
	balances, err := s.GetTokenBalances(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to get token balances: %w", err)
	}

	var tokensToSell []TokenBalance
	for _, balance := range balances {
		// Check if token value is above threshold and it's not USDC
		if balance.UsdValue >= minimumUsdThreshold && balance.Mint != QuoteCurrencyMint {
			tokensToSell = append(tokensToSell, balance)
			s.logger.Printf("Token eligible for sale: %s, Value: $%.2f", balance.Mint, balance.UsdValue)
		}
	}

	return tokensToSell, nil
}

// SwapTokenForSol swaps a token for Wrapped SOL
func (s *SwapService) SwapTokenForSol(ctx context.Context, privateKeyBase58 string, inputMint string, amount uint64) (solana.Signature, error) {
	return s.SwapTokenForSolWithRetries(ctx, privateKeyBase58, inputMint, amount, 10, 3*time.Second)
}

// SwapTokenForSolWithRetries swaps a token for Wrapped SOL with retry mechanism
func (s *SwapService) SwapTokenForSolWithRetries(ctx context.Context, privateKeyBase58 string, inputMint string, amount uint64, maxRetries int, retryDelay time.Duration) (solana.Signature, error) {
	var (
		lastErr           error
		useSharedAccounts = true // Start with shared accounts
	)

	// Get wallet
	wallet, err := s.GetWallet(privateKeyBase58)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get wallet: %w", err)
	}

	// Get public key
	pubKey := wallet.PublicKey()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Step 1: Get quote
		s.logger.Printf("Getting swap quote for %d units of %s -> SOL (attempt %d/%d)...",
			amount, inputMint, attempt, maxRetries)
		quote, err := s.client.GetSwapQuote(inputMint, WrappedSolMint, amount)
		if err != nil {
			lastErr = fmt.Errorf("failed to get swap quote: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		// Calculate output amount in SOL (with 9 decimals)
		outAmountRaw, _ := strconv.ParseUint(quote.OutAmount, 10, 64)
		outAmountFormatted := float64(outAmountRaw) / math.Pow10(9) // SOL has 9 decimals

		s.logger.Printf("Got quote - Will receive: %.5f SOL", outAmountFormatted)

		// Step 2: Get transaction
		s.logger.Printf("Getting swap transaction...")
		swapResp, err := s.client.GetSwapTransactionWithOptions(quote, pubKey, useSharedAccounts)
		if err != nil {
			// Check if it's the specific error about shared accounts
			if strings.Contains(err.Error(), "Simple AMMs are not supported with shared accounts") {
				// If so, try without shared accounts on the next attempt
				useSharedAccounts = false
				s.logger.Printf("Detected Simple AMM error, will retry without shared accounts")
			}

			lastErr = fmt.Errorf("failed to get swap transaction: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		// Step 3: Get blockhash and set it in the transaction
		block, err := sln.BlockhashCache.GetBlockhash(s.solClient)
		if err != nil {
			lastErr = fmt.Errorf("failed to get latest blockhash: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		decodedTx, err := solana.TransactionFromBase64(swapResp.SwapTransaction)
		if err != nil {
			lastErr = fmt.Errorf("failed to decode transaction: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		decodedTx.Message.RecentBlockhash = block.Block.Blockhash

		// Step 4: Sign and send transaction
		s.logger.Printf("Signing and sending transaction...")
		signers := []solana.PrivateKey{wallet}

		if _, err = decodedTx.Sign(
			func(key solana.PublicKey) *solana.PrivateKey {
				for _, payer := range signers {
					if payer.PublicKey().Equals(key) {
						return &payer
					}
				}
				return nil
			},
		); err != nil {
			lastErr = fmt.Errorf("failed to sign transaction: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		sig, err := s.solClient.SendTransactionWithOpts(
			ctx,
			decodedTx,
			rpc.TransactionOpts{
				SkipPreflight: true,
			},
		)
		if err != nil {
			lastErr = fmt.Errorf("failed to send transaction: %w", err)
			s.logger.Printf("Retry %d/%d: %v", attempt, maxRetries, lastErr)
			time.Sleep(retryDelay)
			continue
		}

		// Success! Return the signature
		s.logger.Printf("🎉 Successfully swapped %s for SOL on attempt %d/%d", inputMint, attempt, maxRetries)
		return sig, nil
	}

	// If we get here, all attempts failed
	return solana.Signature{}, fmt.Errorf("all %d attempts failed to swap token: %w", maxRetries, lastErr)
}

// GetSwapTransactionData gets transaction data for a swap without sending it
func (s *SwapService) GetSwapTransactionData(ctx context.Context, inputMint string, outputMint string, amount uint64, userPubKey solana.PublicKey) (string, error) {
	// Step 1: Get quote
	quote, err := s.client.GetSwapQuote(inputMint, outputMint, amount)
	if err != nil {
		return "", fmt.Errorf("failed to get swap quote: %w", err)
	}

	// Step 2: Get transaction
	swapResp, err := s.client.GetSwapTransaction(quote, userPubKey)
	if err != nil {
		return "", fmt.Errorf("failed to get swap transaction: %w", err)
	}

	return swapResp.SwapTransaction, nil
}

// EstimateSwapOutputAmount estimates the amount of SOL to be received when swapping a token
func (s *SwapService) EstimateSwapOutputAmount(tokenMint string, amount uint64) (float64, error) {
	// Get quote
	quote, err := s.client.GetSwapQuote(tokenMint, WrappedSolMint, amount)
	if err != nil {
		return 0, fmt.Errorf("failed to get swap quote: %w", err)
	}

	// Calculate output amount in SOL (with 9 decimals)
	outAmountRaw, _ := strconv.ParseUint(quote.OutAmount, 10, 64)
	outAmountFormatted := float64(outAmountRaw) / math.Pow10(9) // SOL has 9 decimals

	return outAmountFormatted, nil
}
