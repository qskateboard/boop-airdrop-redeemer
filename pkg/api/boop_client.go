package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/models"
)

// BoopClient handles API communication with Boop API
type BoopClient struct {
	config     *config.Config
	httpClient *http.Client
	logger     *log.Logger
}

// NewBoopClient creates a new Boop API client
func NewBoopClient(cfg *config.Config, logger *log.Logger) *BoopClient {
	return &BoopClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// GetPendingAirdrops fetches all pending airdrops for the configured wallet
func (c *BoopClient) GetPendingAirdrops(ctx context.Context) ([]models.AirdropNode, error) {
	query := `
	query GetAccountDistributions($address: String!, $orderBy: StakingAirdropClaimSort, $status: StakingAirdropClaimStatus) {
	  account(address: $address) {
	    stakingAirdrops(orderBy: $orderBy, status: $status) {
	      nodes {
	        ...AccountAirdrop
	      }
	    }
	  }
	}

	fragment AccountAirdrop on AccountStakingAirdrop {
	  id
	  amountLpt
	  amountUsd
	  amountSolLpt
	  proofs
	  claimedAt
	  txHash
	  token {
	    name
	    address
	    symbol
	    logoUrl
	    imageFlag
	  }
	}`

	variables := map[string]string{
		"address": c.config.WalletAddress,
		"orderBy": "AMOUNT_DESC",
		"status":  "PENDING", // Only look for pending claims
	}

	requestBody := models.GraphQLRequest{
		Query:         query,
		Variables:     variables,
		OperationName: "GetAccountDistributions",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error encoding request: %w", err)
	}

	return c.executeGraphQLRequest(ctx, jsonData)
}

// executeGraphQLRequest sends the GraphQL request and handles authentication
func (c *BoopClient) executeGraphQLRequest(ctx context.Context, jsonData []byte) ([]models.AirdropNode, error) {
	// Try with current auth token
	nodes, err := c.doRequest(ctx, jsonData, false)
	if err != nil {
		// If we get an auth error, try refreshing the token and retry
		if isAuthError(err) && c.config.TokenManager != nil {
			c.logger.Println("Authentication error, refreshing token and retrying...")
			refreshErr := c.config.RefreshAuthToken()
			if refreshErr != nil {
				return nil, fmt.Errorf("failed to refresh auth token: %w", refreshErr)
			}

			// Retry with new token
			return c.doRequest(ctx, jsonData, true)
		}
		return nil, err
	}

	return nodes, nil
}

// GraphQLErrorResponse represents a GraphQL response with errors
type GraphQLErrorResponse struct {
	Data struct {
		Account interface{} `json:"account"`
	} `json:"data"`
	Errors []struct {
		Message   string `json:"message"`
		Locations []struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"locations"`
		Path []string `json:"path"`
	} `json:"errors"`
}

// doRequest executes the HTTP request with proper authentication
func (c *BoopClient) doRequest(ctx context.Context, jsonData []byte, isRetry bool) ([]models.AirdropNode, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.GraphQLURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use token manager if available
	if c.config.TokenManager != nil {
		req.Header.Set("Authorization", c.config.GetAuthToken())
	} else {
		req.Header.Set("Authorization", c.config.AuthToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	bodyBytes := new(bytes.Buffer)
	_, err = bodyBytes.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Check for HTTP error status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, bodyBytes.String())
	}

	// Check for GraphQL auth errors (which may come with 200 status code)
	if hasGraphQLAuthError(bodyBytes.Bytes()) {
		return nil, fmt.Errorf("GraphQL authorization error: %s", bodyBytes.String())
	}

	// Parse the response for normal processing
	var response models.GraphQLResponse
	if err := json.Unmarshal(bodyBytes.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return response.Data.Account.StakingAirdrops.Nodes, nil
}

// hasGraphQLAuthError checks if the response contains GraphQL auth errors
func hasGraphQLAuthError(responseBody []byte) bool {
	var errorResp GraphQLErrorResponse
	if err := json.Unmarshal(responseBody, &errorResp); err != nil {
		return false
	}

	// Check if we have errors related to authorization
	for _, err := range errorResp.Errors {
		if strings.Contains(strings.ToLower(err.Message), "not authorized") ||
			strings.Contains(strings.ToLower(err.Message), "unauthorized") {
			return true
		}
	}

	return false
}

// isAuthError checks if the error is related to authentication
func isAuthError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Check for standard HTTP auth errors
	if strings.Contains(errMsg, "unexpected status code: 401") ||
		strings.Contains(errMsg, "unexpected status code: 403") {
		return true
	}

	// Check for GraphQL auth errors
	if strings.Contains(errMsg, "graphql authorization error") ||
		strings.Contains(errMsg, "user not authorized") {
		return true
	}

	return false
}
