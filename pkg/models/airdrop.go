package models

// Token represents token information from Boop API
type Token struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Symbol    string `json:"symbol"`
	LogoURL   string `json:"logoUrl"`
	ImageFlag string `json:"imageFlag"`
}

// AirdropNode represents a single airdrop from Boop API
type AirdropNode struct {
	ID           string          `json:"id"`
	AmountLpt    string          `json:"amountLpt"`
	AmountUsd    string          `json:"amountUsd"`
	AmountSolLpt string          `json:"amountSolLpt"`
	Proofs       [][]interface{} `json:"proofs"`    // Using interface{} as proofs structure isn't important for finding new airdrops
	ClaimedAt    interface{}     `json:"claimedAt"` // Can be null
	TxHash       interface{}     `json:"txHash"`    // Can be null
	Token        Token           `json:"token"`
}

// GraphQLRequest represents the structure for GraphQL API requests
type GraphQLRequest struct {
	Query         string            `json:"query"`
	Variables     map[string]string `json:"variables"`
	OperationName string            `json:"operationName"`
}

// GraphQLResponse represents the structure for GraphQL API responses
type GraphQLResponse struct {
	Data ResponseData `json:"data"`
}

// ResponseData represents the account data in API response
type ResponseData struct {
	Account AccountData `json:"account"`
}

// AccountData represents user account information
type AccountData struct {
	StakingAirdrops StakingAirdropsData `json:"stakingAirdrops"`
}

// StakingAirdropsData represents a collection of airdrops
type StakingAirdropsData struct {
	Nodes []AirdropNode `json:"nodes"`
}
