package jupiter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Client represents a Jupiter API client
type Client struct {
	httpClient *http.Client
	logger     *log.Logger
}

// NewClient creates a new Jupiter API client
func NewClient(logger *log.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// GetPrices fetches USD prices for given token mints
func (c *Client) GetPrices(tokenMints []string) (map[string]float64, error) {
	if len(tokenMints) == 0 {
		return make(map[string]float64), nil
	}

	idsParam := strings.Join(tokenMints, ",")
	url := fmt.Sprintf("%s?ids=%s&vsToken=%s", JupiterPriceAPI, idsParam, QuoteCurrencyMint) // Price vs USDC

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call Jupiter Price API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jupiter Price API returned non-OK status: %d", resp.StatusCode)
	}

	var priceResp PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jupiter Price API response: %w", err)
	}

	prices := make(map[string]float64)
	for mint, data := range priceResp.Data {
		prices[mint] = data.Price
	}
	return prices, nil
}

// GetSwapQuote fetches a swap quote from Jupiter
func (c *Client) GetSwapQuote(inputMint, outputMint string, amount uint64) (*QuoteResponse, error) {
	url := fmt.Sprintf("%s?inputMint=%s&outputMint=%s&amount=%d&slippageBps=%d&onlyDirectRoutes=false",
		JupiterQuoteAPI, inputMint, outputMint, amount, SlippageBps)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call Jupiter Quote API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("Jupiter Quote API returned non-OK status: %d - %s", resp.StatusCode, buf.String())
	}

	var quoteResp QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jupiter Quote API response: %w", err)
	}

	// Basic validation: Check if we got a valid quote (outAmount > 0)
	outAmount, _ := strconv.ParseUint(quoteResp.OutAmount, 10, 64)
	if outAmount == 0 {
		return nil, fmt.Errorf("Jupiter Quote API returned a quote with 0 output amount for %s -> %s", inputMint, outputMint)
	}

	return &quoteResp, nil
}

// GetSwapTransaction gets a transaction for a swap with the given quote
func (c *Client) GetSwapTransaction(quote *QuoteResponse, userPubKey solana.PublicKey) (*SwapResponse, error) {
	return c.GetSwapTransactionWithOptions(quote, userPubKey, true)
}

// GetSwapTransactionWithOptions gets a transaction for a swap with options
func (c *Client) GetSwapTransactionWithOptions(quote *QuoteResponse, userPubKey solana.PublicKey, useSharedAccounts bool) (*SwapResponse, error) {
	swapReq := SwapRequest{
		UserPublicKey:                 userPubKey.String(),
		QuoteResponse:                 *quote,
		WrapAndUnwrapSol:              true,              // Automatically handle SOL wrapping/unwrapping if needed
		UseSharedAccounts:             useSharedAccounts, // Use Jupiter's shared accounts (can be turned off)
		AsLegacyTransaction:           false,             // Use Versioned Transactions by default
		ComputeUnitPriceMicroLamports: 300000,            // Optional: Add priority fee here if desired
	}

	jsonData, err := json.Marshal(swapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal swap request: %w", err)
	}

	resp, err := c.httpClient.Post(JupiterSwapAPI, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Jupiter Swap API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("Jupiter Swap API returned non-OK status: %d - %s", resp.StatusCode, buf.String())
	}

	var swapResp SwapResponse
	if err := json.NewDecoder(resp.Body).Decode(&swapResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jupiter Swap API response: %w", err)
	}

	if swapResp.SwapTransaction == "" {
		return nil, fmt.Errorf("Jupiter Swap API returned empty transaction")
	}

	return &swapResp, nil
}

// SignAndSendTransaction signs and sends the swap transaction
func (c *Client) SignAndSendTransaction(ctx context.Context, solClient *rpc.Client, encodedTx string, wallet solana.PrivateKey) (solana.Signature, error) {
	// Decode the base64 encoded transaction
	txBytes, err := base64.StdEncoding.DecodeString(encodedTx)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to decode base64 transaction: %w", err)
	}

	// For simplicity, we'll treat all transactions as legacy transactions since versioned transactions
	// require additional handling that may require more dependencies
	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to deserialize transaction from bytes: %w", err)
	}

	// Sign the transaction
	recent, err := solClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get recent blockhash: %w", err)
	}
	tx.Message.RecentBlockhash = recent.Value.Blockhash

	// Sign with the wallet private key
	signers := []solana.PrivateKey{wallet}
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		for _, signer := range signers {
			if signer.PublicKey().Equals(key) {
				return &signer
			}
		}
		return nil
	})

	// Send the transaction
	sig, err := solClient.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return sig, nil
}
