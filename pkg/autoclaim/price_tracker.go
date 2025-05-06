package autoclaim

import (
	"strconv"
	"sync"
	"time"

	"boop-airdrop-redeemer/pkg/models"
)

// TokenPriceInfo stores price tracking information for a token
type TokenPriceInfo struct {
	LastPrice     float64
	LastChanged   time.Time
	FirstObserved time.Time
}

// PriceTracker tracks token price history and stability
type PriceTracker struct {
	tokenPriceHistory map[string]TokenPriceInfo
	mutex             *sync.Mutex
}

// NewPriceTracker creates a new price tracker
func NewPriceTracker() *PriceTracker {
	return &PriceTracker{
		tokenPriceHistory: make(map[string]TokenPriceInfo),
		mutex:             &sync.Mutex{},
	}
}

// UpdatePriceData updates the price data for a given airdrop
func (p *PriceTracker) UpdatePriceData(airdrop models.AirdropNode) {
	// Parse USD value
	usdValue, err := strconv.ParseFloat(airdrop.AmountUsd, 64)
	if err != nil {
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	priceKey := airdrop.ID

	// Check if we've seen this token before
	priceInfo, exists := p.tokenPriceHistory[priceKey]

	if !exists {
		// First time seeing this token
		p.tokenPriceHistory[priceKey] = TokenPriceInfo{
			LastPrice:     usdValue,
			LastChanged:   now,
			FirstObserved: now,
		}
	} else if priceInfo.LastPrice != usdValue {
		// Price changed
		p.tokenPriceHistory[priceKey] = TokenPriceInfo{
			LastPrice:     usdValue,
			LastChanged:   now,
			FirstObserved: priceInfo.FirstObserved,
		}
	}
}

// GetTokenPriceInfo returns price info for a token
func (p *PriceTracker) GetTokenPriceInfo(tokenID string) *TokenPriceInfo {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if info, exists := p.tokenPriceHistory[tokenID]; exists {
		return &TokenPriceInfo{
			LastPrice:     info.LastPrice,
			LastChanged:   info.LastChanged,
			FirstObserved: info.FirstObserved,
		}
	}

	return nil
}
