package jupiter

// API endpoints for Jupiter (Solana DEX aggregator)
const (
	// Jupiter API V6 Endpoints
	JupiterQuoteAPI = "https://quote-api.jup.ag/v6/quote"
	JupiterSwapAPI  = "https://quote-api.jup.ag/v6/swap"
	JupiterPriceAPI = "https://price.jup.ag/v4/price" // Price API

	// Configuration
	QuoteCurrencyMint = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v" // USDC Mint on Mainnet
	WrappedSolMint    = "So11111111111111111111111111111111111111112"  // Wrapped SOL Mint on Mainnet
	SlippageBps       = 1000                                           // 10% slippage tolerance (1000 basis points)
)
