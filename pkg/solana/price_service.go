package solana

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// PriceResponse represents a response from a price API
type PriceResponse struct {
	Solana struct {
		Usd float64 `json:"usd"`
	} `json:"solana"`
}

// PriceService tracks the current SOL price
type PriceService struct {
	currentPrice   float64
	lastUpdated    time.Time
	updateInterval time.Duration
	mu             sync.RWMutex
	logger         *log.Logger
	apiURL         string
	stopChan       chan struct{}
}

// NewPriceService creates a new price service
func NewPriceService(logger *log.Logger) *PriceService {
	return &PriceService{
		currentPrice:   0,
		updateInterval: 10 * time.Minute,
		logger:         logger,
		apiURL:         "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=usd",
		stopChan:       make(chan struct{}),
	}
}

// Start begins the price update service
func (p *PriceService) Start() {
	// Fetch price immediately
	p.updatePrice()

	// Start update loop
	go func() {
		ticker := time.NewTicker(p.updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.updatePrice()
			case <-p.stopChan:
				return
			}
		}
	}()
}

// Stop terminates the price update service
func (p *PriceService) Stop() {
	close(p.stopChan)
}

// GetCurrentPrice returns the current SOL price
func (p *PriceService) GetCurrentPrice() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// If price is not available or too old, update it synchronously
	if p.currentPrice == 0 || time.Since(p.lastUpdated) > 30*time.Minute {
		p.mu.RUnlock() // Unlock before updating
		p.updatePrice()
		p.mu.RLock() // Lock again to read the updated price
	}

	return p.currentPrice
}

// updatePrice fetches the latest SOL price from the API
func (p *PriceService) updatePrice() {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(p.apiURL)
	if err != nil {
		p.logger.Printf("Error fetching SOL price: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.logger.Printf("Error fetching SOL price: HTTP %d", resp.StatusCode)
		return
	}

	var priceData PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceData); err != nil {
		p.logger.Printf("Error parsing SOL price data: %v", err)
		return
	}

	p.mu.Lock()
	p.currentPrice = priceData.Solana.Usd
	p.lastUpdated = time.Now()
	p.mu.Unlock()

	p.logger.Printf("Updated SOL price: $%.2f", p.currentPrice)
}
