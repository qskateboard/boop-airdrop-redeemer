package jupiter

// PriceResponse represents the Jupiter Price API response
type PriceResponse struct {
	Data map[string]struct {
		Price         float64 `json:"price"`
		MintSymbol    string  `json:"mintSymbol"`
		VsTokenSymbol string  `json:"vsTokenSymbol"`
	} `json:"data"`
	TimeTaken float64 `json:"timeTaken"`
}

// QuoteResponse represents the Jupiter Quote API response
type QuoteResponse struct {
	InputMint            string `json:"inputMint"`
	InAmount             string `json:"inAmount"`
	OutputMint           string `json:"outputMint"`
	OutAmount            string `json:"outAmount"`
	OtherAmountThreshold string `json:"otherAmountThreshold"`
	SwapMode             string `json:"swapMode"`
	SlippageBps          int    `json:"slippageBps"`
	PlatformFee          *struct {
		Amount string `json:"amount"`
		Mint   string `json:"mint"`
	} `json:"platformFee"`
	PriceImpactPct string `json:"priceImpactPct"`
	RoutePlan      []struct {
		SwapInfo struct {
			AmmKey     string `json:"ammKey"`
			Label      string `json:"label"`
			InputMint  string `json:"inputMint"`
			OutputMint string `json:"outputMint"`
			InAmount   string `json:"inAmount"`
			OutAmount  string `json:"outAmount"`
			FeeAmount  string `json:"feeAmount"`
			FeeMint    string `json:"feeMint"`
		} `json:"swapInfo"`
		Percent int `json:"percent"`
	} `json:"routePlan"`
	ContextSlot uint64  `json:"contextSlot"`
	TimeTaken   float64 `json:"timeTaken"`
}

// SwapRequest represents the Jupiter Swap API request
type SwapRequest struct {
	UserPublicKey                 string        `json:"userPublicKey"`
	QuoteResponse                 QuoteResponse `json:"quoteResponse"`
	WrapAndUnwrapSol              bool          `json:"wrapAndUnwrapSol"`
	UseSharedAccounts             bool          `json:"useSharedAccounts"`
	FeeAccount                    string        `json:"feeAccount,omitempty"`                    // Optional: specific fee account
	ComputeUnitPriceMicroLamports int           `json:"computeUnitPriceMicroLamports,omitempty"` // Optional: priority fees
	AsLegacyTransaction           bool          `json:"asLegacyTransaction,omitempty"`           // Set to true for legacy tx format if needed
}

// SwapResponse represents the Jupiter Swap API response
type SwapResponse struct {
	SwapTransaction string `json:"swapTransaction"` // base64 encoded transaction
	LastErrorId     string `json:"lastErrorId"`
	LastErrorTs     int64  `json:"lastErrorTs"`
}

// TokenBalance represents a token balance with its price information
type TokenBalance struct {
	Mint     string
	Amount   uint64
	Decimals uint8
	UiAmount float64
	UsdPrice float64
	UsdValue float64
}
