package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"boop-airdrop-redeemer/pkg/api"
	"boop-airdrop-redeemer/pkg/config"
	"boop-airdrop-redeemer/pkg/models"
)

// AirdropStore is an interface for storing and checking airdrops
type AirdropStore interface {
	SaveAirdrop(airdrop models.AirdropNode)
	HasAirdropWithID(id string) bool
	GetAllAirdrops() []models.AirdropNode
	ClaimAirdrop(airdropID string) (models.AirdropNode, error)
}

// AirdropMonitor monitors for new airdrops
type AirdropMonitor struct {
	client    *api.BoopClient
	store     AirdropStore
	config    *config.Config
	stopCh    chan struct{}
	waitGroup sync.WaitGroup
	logger    *log.Logger
}

// inMemoryAirdropStore is a simple in-memory implementation of AirdropStore
type inMemoryAirdropStore struct {
	airdrops map[string]models.AirdropNode
	mu       sync.RWMutex
}

// NewInMemoryAirdropStore creates a new in-memory store for airdrops
func NewInMemoryAirdropStore() AirdropStore {
	return &inMemoryAirdropStore{
		airdrops: make(map[string]models.AirdropNode),
	}
}

// SaveAirdrop stores an airdrop in memory
func (s *inMemoryAirdropStore) SaveAirdrop(airdrop models.AirdropNode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.airdrops[airdrop.ID] = airdrop
}

// HasAirdropWithID checks if an airdrop with given ID exists
func (s *inMemoryAirdropStore) HasAirdropWithID(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.airdrops[id]
	return exists
}

// GetAllAirdrops returns all stored airdrops
func (s *inMemoryAirdropStore) GetAllAirdrops() []models.AirdropNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	airdrops := make([]models.AirdropNode, 0, len(s.airdrops))
	for _, airdrop := range s.airdrops {
		airdrops = append(airdrops, airdrop)
	}
	return airdrops
}

// ClaimAirdrop retrieves an airdrop by ID to claim it
func (s *inMemoryAirdropStore) ClaimAirdrop(airdropID string) (models.AirdropNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	airdrop, exists := s.airdrops[airdropID]
	if !exists {
		return models.AirdropNode{}, fmt.Errorf("airdrop with ID %s not found", airdropID)
	}

	return airdrop, nil
}

// NewAirdropMonitor creates a new monitor with the provided dependencies
func NewAirdropMonitor(client *api.BoopClient, store AirdropStore, cfg *config.Config, logger *log.Logger) *AirdropMonitor {
	return &AirdropMonitor{
		client:    client,
		store:     store,
		config:    cfg,
		stopCh:    make(chan struct{}),
		waitGroup: sync.WaitGroup{},
		logger:    logger,
	}
}

// Start begins the monitoring process
func (m *AirdropMonitor) Start(ctx context.Context) error {
	m.logger.Println("Starting airdrop monitor...")
	m.waitGroup.Add(1)

	// First check immediately
	if err := m.checkAirdrops(ctx); err != nil {
		m.logger.Printf("Error in initial airdrop check: %v", err)
	}

	go func() {
		defer m.waitGroup.Done()
		ticker := time.NewTicker(m.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.checkAirdrops(ctx); err != nil {
					m.logger.Printf("Error checking airdrops: %v", err)
				}
			case <-m.stopCh:
				m.logger.Println("Stopping airdrop monitor...")
				return
			case <-ctx.Done():
				m.logger.Println("Context cancelled, stopping airdrop monitor...")
				return
			}
		}
	}()

	return nil
}

// Stop gracefully stops the monitoring process
func (m *AirdropMonitor) Stop() {
	close(m.stopCh)
	m.waitGroup.Wait()
}

// checkAirdrops checks for new airdrops and processes them
func (m *AirdropMonitor) checkAirdrops(ctx context.Context) error {
	m.logger.Printf("[%s] Checking for new airdrops...", time.Now().Format("15:04:05"))

	airdrops, err := m.client.GetPendingAirdrops(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch airdrops: %w", err)
	}

	foundNew := false
	for _, airdrop := range airdrops {
		if !m.store.HasAirdropWithID(airdrop.ID) {
			// Found a new airdrop!
			m.processNewAirdrop(airdrop)
			m.store.SaveAirdrop(airdrop)
			foundNew = true
		}
	}

	if !foundNew {
		m.logger.Println("No new airdrops found.")
	}

	return nil
}

// processNewAirdrop handles notification for a newly discovered airdrop
func (m *AirdropMonitor) processNewAirdrop(airdrop models.AirdropNode) {
	m.logger.Printf(">>> NEW AIRDROP DETECTED! <<<")

	// Convert string amounts to float64
	amountLpt, _ := strconv.ParseFloat(airdrop.AmountLpt, 64)
	amountUsd, _ := strconv.ParseFloat(airdrop.AmountUsd, 64)

	// Format with requested formatting
	formattedLpt := amountLpt / 1e9

	m.logger.Printf("  - ID: %s, Token: %s (%s), Amount LPT: %.2f, USD: ~$%.2f",
		airdrop.ID, airdrop.Token.Name, airdrop.Token.Symbol, formattedLpt, amountUsd)

	// Here you could add additional notification methods
	// For example: send email, push notification, etc.
}
